// Package deck computes simple stats and role coverage for a Clash Royale deck.
package deck

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"clash-analyzer/internal/clashapi"
)

// Role is a job a card can fill in a deck.
type Role string

const (
	WinCondition Role = "win_condition"
	SmallSpell   Role = "small_spell" // cheap spell for swarms: Zap, Log, Snowball...
	BigSpell     Role = "big_spell"   // heavy damage spell: Fireball, Poison, Rocket...
	UtilitySpell Role = "spell"       // spell that isn't primarily damage: Freeze, Rage...
	Building     Role = "building"
	AirDefense   Role = "air_defense" // can damage air units, directly or via spawns
)

// rolesJSON maps API card names to their roles. It is hand-maintained;
// cards missing from it show up in Analysis.UnknownCards.
//
//go:embed roles.json
var rolesJSON []byte

var cardRoles = mustLoadRoles(rolesJSON)

func mustLoadRoles(data []byte) map[string][]Role {
	var raw map[string][]Role
	if err := json.Unmarshal(data, &raw); err != nil {
		panic(fmt.Sprintf("deck: parsing roles.json: %v", err))
	}
	// Key by lowercase name so lookups tolerate casing differences.
	roles := make(map[string][]Role, len(raw))
	for name, r := range raw {
		roles[strings.ToLower(name)] = r
	}
	return roles
}

// RolesFor returns a card's roles. ok is false if the card isn't in roles.json;
// a known card may still have no roles (e.g. Knight).
func RolesFor(name string) (roles []Role, ok bool) {
	roles, ok = cardRoles[strings.ToLower(name)]
	return roles, ok
}

type CardInfo struct {
	Name   string `json:"name"`
	Elixir int    `json:"elixir"` // 0 when the cost is variable (Mirror)
	Roles  []Role `json:"roles"`
}

type Analysis struct {
	Cards         []CardInfo `json:"cards"`
	AverageElixir float64    `json:"averageElixir" jsonschema:"mean elixir cost, rounded to one decimal like the in-game display"`
	FourCardCycle int        `json:"fourCardCycle" jsonschema:"total elixir of the four cheapest cards: the cost to cycle back to a given card"`
	CycleCards    []string   `json:"cycleCards" jsonschema:"the four cheapest cards"`
	WinConditions []string   `json:"winConditions"`
	SmallSpells   []string   `json:"smallSpells"`
	BigSpells     []string   `json:"bigSpells"`
	OtherSpells   []string   `json:"otherSpells"`
	Buildings     []string   `json:"buildings"`
	AirDefense    []string   `json:"airDefense"`
	Gaps          []string   `json:"gaps" jsonschema:"missing roles or other structural problems"`
	UnknownCards  []string   `json:"unknownCards,omitempty" jsonschema:"cards with no role data; gaps may be false positives if these fill a role"`
}

// Analyze computes elixir stats and role coverage for a deck.
func Analyze(cards []clashapi.Card) Analysis {
	a := Analysis{
		Cards: make([]CardInfo, 0, len(cards)),
		// Non-nil so empty lists encode as [] rather than null.
		CycleCards:    []string{},
		WinConditions: []string{},
		SmallSpells:   []string{},
		BigSpells:     []string{},
		OtherSpells:   []string{},
		Buildings:     []string{},
		AirDefense:    []string{},
		Gaps:          []string{},
	}

	var costed []CardInfo  // cards with a fixed elixir cost
	var mirrors []CardInfo // cards without one (Mirror)
	for _, c := range cards {
		roles, ok := RolesFor(c.Name)
		if !ok {
			a.UnknownCards = append(a.UnknownCards, c.Name)
		}
		if roles == nil {
			roles = []Role{}
		}
		info := CardInfo{Name: c.Name, Elixir: c.ElixirCost, Roles: roles}
		a.Cards = append(a.Cards, info)
		if c.ElixirCost > 0 {
			costed = append(costed, info)
		} else {
			mirrors = append(mirrors, info)
		}

		for _, r := range roles {
			switch r {
			case WinCondition:
				a.WinConditions = append(a.WinConditions, c.Name)
			case SmallSpell:
				a.SmallSpells = append(a.SmallSpells, c.Name)
			case BigSpell:
				a.BigSpells = append(a.BigSpells, c.Name)
			case UtilitySpell:
				a.OtherSpells = append(a.OtherSpells, c.Name)
			case Building:
				a.Buildings = append(a.Buildings, c.Name)
			case AirDefense:
				a.AirDefense = append(a.AirDefense, c.Name)
			}
		}
	}

	if len(costed) > 0 {
		// Mirror is left out of the average.
		total := 0
		for _, c := range costed {
			total += c.Elixir
		}
		a.AverageElixir = math.Round(float64(total)/float64(len(costed))*10) / 10

		// For the cycle, Mirror counts as the cheapest card in the deck.
		slices.SortStableFunc(costed, func(x, y CardInfo) int { return x.Elixir - y.Elixir })
		pool := costed
		for _, m := range mirrors {
			m.Elixir = costed[0].Elixir
			pool = append(pool, m)
		}
		slices.SortStableFunc(pool, func(x, y CardInfo) int { return x.Elixir - y.Elixir })
		for _, c := range pool[:min(4, len(pool))] {
			a.FourCardCycle += c.Elixir
			a.CycleCards = append(a.CycleCards, c.Name)
		}
	}

	a.Gaps = findGaps(a, len(cards))
	return a
}

func findGaps(a Analysis, deckSize int) []string {
	gaps := []string{}
	if deckSize != 8 {
		gaps = append(gaps, fmt.Sprintf("deck has %d cards, expected 8", deckSize))
	}
	if len(a.WinConditions) == 0 {
		gaps = append(gaps, "no win condition")
	}
	if len(a.SmallSpells)+len(a.BigSpells)+len(a.OtherSpells) == 0 {
		gaps = append(gaps, "no spells")
	} else {
		if len(a.SmallSpells) == 0 {
			gaps = append(gaps, "no small spell (vulnerable to swarms like Goblin Barrel, Skeleton Army)")
		}
		if len(a.BigSpells) == 0 {
			gaps = append(gaps, "no big spell (hard to finish towers or punish support troops)")
		}
	}
	switch len(a.AirDefense) {
	case 0:
		gaps = append(gaps, "no air defense (vulnerable to Balloon, Lava Hound, Minion Horde)")
	case 1:
		gaps = append(gaps, "only one air defense ("+a.AirDefense[0]+")")
	}
	return gaps
}
