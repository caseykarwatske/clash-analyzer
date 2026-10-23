package deck

import (
	"fmt"
	"slices"
	"testing"

	"clash-analyzer/internal/clashapi"
)

var (
	hogCycle = []string{"Hog Rider", "Musketeer", "Cannon", "Ice Golem", "Ice Spirit", "Skeletons", "Fireball", "The Log"}
	logBait  = []string{"Goblin Barrel", "Princess", "Knight", "Inferno Tower", "Rocket", "The Log", "Ice Spirit", "Goblin Gang"}
	lavaLoon = []string{"Lava Hound", "Balloon", "Mega Minion", "Minions", "Tombstone", "Arrows", "Fireball", "Skeleton Dragons"}
)

var battleSeq int

// battle builds a standard Path of Legend battle between two players.
func battle(tag string, myDeck []string, myCrowns int, oppTag string, oppDeck []string, oppCrowns int) clashapi.Battle {
	battleSeq++
	toCards := func(names []string) []clashapi.Card {
		cards := make([]clashapi.Card, len(names))
		for i, n := range names {
			cards[i] = clashapi.Card{Name: n}
		}
		return cards
	}
	return clashapi.Battle{
		Type:          "pathOfLegend",
		BattleTime:    fmt.Sprintf("20261022T%06d.000Z", battleSeq),
		DeckSelection: "collection",
		Team:          []clashapi.BattlePlayer{{Tag: tag, Crowns: myCrowns, Cards: toCards(myDeck)}},
		Opponent:      []clashapi.BattlePlayer{{Tag: oppTag, Crowns: oppCrowns, Cards: toCards(oppDeck)}},
	}
}

func TestSummarizeFromOpponentSide(t *testing.T) {
	b := battle("#AAA", hogCycle, 1, "#BBB", logBait, 3)
	s := Summarize(b, "bbb")
	if s.Result != Win || s.Crowns != 3 || !SameDeck(s.Deck, logBait) || s.OpponentTag != "#AAA" {
		t.Errorf("Summarize from #BBB = %+v", s)
	}
}

func TestIsStandard1v1(t *testing.T) {
	b := battle("#A", hogCycle, 1, "#B", logBait, 0)
	if !IsStandard1v1(b) {
		t.Error("path of legend battle not standard")
	}
	b.Type = "friendly"
	if IsStandard1v1(b) {
		t.Error("friendly counted as standard")
	}
}

func TestAnalyzeLosses(t *testing.T) {
	var battles []clashapi.Battle
	// Loses every game to Lava Loon, wins every game against Log Bait.
	for range 4 {
		battles = append(battles, battle("#ME", hogCycle, 0, "#X", lavaLoon, 1))
		battles = append(battles, battle("#ME", hogCycle, 2, "#Y", logBait, 1))
	}
	// A loss with a different deck, excluded when filtering by deck.
	other := slices.Clone(hogCycle)
	other[0] = "Royal Giant"
	battles = append(battles, battle("#ME", other, 0, "#Y", logBait, 1))

	r := AnalyzeLosses(battles, "#ME", hogCycle)
	if r.Battles != 8 || r.Wins != 4 || r.Losses != 4 || r.LossRate != 0.5 {
		t.Fatalf("totals = %d battles %d-%d, loss rate %v", r.Battles, r.Wins, r.Losses, r.LossRate)
	}
	// Every Lava Loon card is a threat; no Log Bait card is.
	for _, th := range r.Threats {
		if !slices.Contains(lavaLoon, th.Card) {
			t.Errorf("unexpected threat %+v", th)
		}
		if th.LossRate != 1 || th.ExcessLosses != 2 {
			t.Errorf("threat %s: loss rate %v, excess %v; want 1, 2", th.Card, th.LossRate, th.ExcessLosses)
		}
	}
	if len(r.Threats) != len(lavaLoon) {
		t.Errorf("got %d threats, want %d", len(r.Threats), len(lavaLoon))
	}
	// Worst win conditions first.
	if len(r.WinConditions) != 3 || r.WinConditions[0].LossRate != 1 || r.WinConditions[2].Card != "Goblin Barrel" {
		t.Errorf("WinConditions = %+v", r.WinConditions)
	}

	if all := AnalyzeLosses(battles, "#ME", nil); all.Battles != 9 {
		t.Errorf("all decks: %d battles, want 9", all.Battles)
	}
}

func TestMetaAndCounters(t *testing.T) {
	var battles []clashapi.Battle
	for range 3 {
		// Lava Loon beats Hog Cycle; Log Bait beats Lava Loon.
		battles = append(battles, battle("#A", lavaLoon, 3, "#B", hogCycle, 0))
		battles = append(battles, battle("#C", logBait, 1, "#A", lavaLoon, 0))
	}
	// The same battle seen from the other player's log is not double-counted.
	dup := battles[0]
	dup.Team, dup.Opponent = dup.Opponent, dup.Team
	battles = append(battles, dup)

	m := MetaFromBattles(battles, 3, 10)
	if m.Battles != 6 {
		t.Fatalf("Battles = %d, want 6", m.Battles)
	}
	// Fireball is in both Lava Loon (3-3) and Hog Cycle (0-3).
	if m.Popular[0].Card != "Fireball" || m.Popular[0].Uses != 9 || m.Popular[0].Wins != 3 {
		t.Errorf("Popular[0] = %+v", m.Popular[0])
	}
	if len(m.Decks) != 3 || m.Decks[0].Uses != 6 {
		t.Errorf("Decks = %+v", m.Decks)
	}

	c := Counters(battles, []string{"Hog Rider"}, 3, 5)[0]
	if c.Games != 3 || c.WinRateAgainst != 1 {
		t.Errorf("Hog Rider baseline = %d games, %v", c.Games, c.WinRateAgainst)
	}
	if len(c.Counters) == 0 || !slices.Contains(lavaLoon, c.Counters[0].Card) || c.Counters[0].WinRate != 1 {
		t.Errorf("Hog Rider counters = %+v", c.Counters)
	}
}
