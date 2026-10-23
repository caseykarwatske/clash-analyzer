package deck

import (
	"slices"
	"testing"

	"clash-analyzer/internal/clashapi"
)

func cards(spec map[string]int) []clashapi.Card {
	var out []clashapi.Card
	for name, cost := range spec {
		out = append(out, clashapi.Card{Name: name, ElixirCost: cost})
	}
	return out
}

func TestAnalyzeHogCycle(t *testing.T) {
	// Classic 2.6 Hog Cycle.
	a := Analyze(cards(map[string]int{
		"Hog Rider": 4, "Musketeer": 4, "Cannon": 3, "Ice Golem": 2,
		"Ice Spirit": 1, "Skeletons": 1, "Fireball": 4, "The Log": 2,
	}))

	if a.AverageElixir != 2.6 {
		t.Errorf("AverageElixir = %v, want 2.6", a.AverageElixir)
	}
	if a.FourCardCycle != 6 { // 1 + 1 + 2 + 2
		t.Errorf("FourCardCycle = %d, want 6", a.FourCardCycle)
	}
	if !slices.Equal(a.WinConditions, []string{"Hog Rider"}) {
		t.Errorf("WinConditions = %v", a.WinConditions)
	}
	if !slices.Equal(a.Buildings, []string{"Cannon"}) {
		t.Errorf("Buildings = %v", a.Buildings)
	}
	if len(a.Gaps) != 0 {
		t.Errorf("Gaps = %v, want none", a.Gaps)
	}
	if len(a.UnknownCards) != 0 {
		t.Errorf("UnknownCards = %v, want none", a.UnknownCards)
	}
}

func TestAnalyzeFlagsGaps(t *testing.T) {
	a := Analyze(cards(map[string]int{
		"Golem": 8, "Lightning": 6, "Cannon": 3, "Hog Rider": 4,
	}))

	for _, want := range []string{
		"deck has 4 cards, expected 8",
		"no small spell (vulnerable to swarms like Goblin Barrel, Skeleton Army)",
		"no air defense (vulnerable to Balloon, Lava Hound, Minion Horde)",
	} {
		if !slices.Contains(a.Gaps, want) {
			t.Errorf("Gaps = %v, missing %q", a.Gaps, want)
		}
	}
}

func TestAnalyzeMirror(t *testing.T) {
	a := Analyze(cards(map[string]int{
		"Mirror": 0, "Zap": 2, "Rocket": 6, "Hog Rider": 4, "Musketeer": 4,
	}))
	// Average skips Mirror: (2+6+4+4)/4.
	if a.AverageElixir != 4 {
		t.Errorf("AverageElixir = %v, want 4", a.AverageElixir)
	}
	// Mirror counts as the cheapest card (Zap, 2): 2 + 2 + 4 + 4.
	if a.FourCardCycle != 12 || !slices.Contains(a.CycleCards, "Mirror") {
		t.Errorf("FourCardCycle = %d over %v, want 12 including Mirror", a.FourCardCycle, a.CycleCards)
	}
}

func TestRolesForIsCaseInsensitive(t *testing.T) {
	if r, ok := RolesFor("x-bow"); !ok || !slices.Contains(r, AirDefense) {
		t.Errorf("RolesFor(x-bow) = %v, %v", r, ok)
	}
	if _, ok := RolesFor("Not A Card"); ok {
		t.Error("RolesFor(Not A Card) reported ok")
	}
}
