package deck

import (
	"cmp"
	"fmt"
	"slices"

	"clash-analyzer/internal/clashapi"
)

// CardThreat is how a player fares against decks containing a given card.
type CardThreat struct {
	Card     string  `json:"card"`
	Roles    []Role  `json:"roles"`
	Faced    int     `json:"faced" jsonschema:"battles against a deck containing this card"`
	Losses   int     `json:"losses"`
	LossRate float64 `json:"lossRate"`
	// ExcessLosses is losses beyond what the player's overall loss rate
	// predicts, so a card faced often and lost to often ranks above a card
	// lost to once.
	ExcessLosses float64 `json:"excessLosses" jsonschema:"losses above what the overall loss rate predicts; higher is a bigger problem"`
}

type WeaknessReport struct {
	Battles  int     `json:"battles" jsonschema:"standard 1v1 battles analyzed"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	Draws    int     `json:"draws"`
	LossRate float64 `json:"lossRate"`
	// Threats are the opponent cards most associated with losses.
	Threats []CardThreat `json:"threats" jsonschema:"opponent cards most associated with losses, worst first"`
	// WinConditions covers every opposing win condition faced, worst first.
	WinConditions []CardThreat `json:"winConditions" jsonschema:"results against each opposing win condition, worst first"`
	Notes         []string     `json:"notes"`
}

const (
	minFacedForThreat = 2
	maxThreats        = 10
)

// AnalyzeLosses finds which opponent cards a player struggles against,
// using standard 1v1 battles from their log. If deck is non-nil, only
// battles played with that deck (in any order) are counted.
func AnalyzeLosses(battles []clashapi.Battle, tag string, deck []string) WeaknessReport {
	r := WeaknessReport{Threats: []CardThreat{}, WinConditions: []CardThreat{}, Notes: []string{}}

	faced := map[string]int{}
	lost := map[string]int{}
	for _, b := range battles {
		if !IsStandard1v1(b) {
			continue
		}
		s := Summarize(b, tag)
		if deck != nil && !SameDeck(s.Deck, deck) {
			continue
		}
		r.Battles++
		switch s.Result {
		case Win:
			r.Wins++
		case Loss:
			r.Losses++
		case Draw:
			r.Draws++
		}
		for _, c := range s.OpponentDeck {
			faced[c]++
			if s.Result == Loss {
				lost[c]++
			}
		}
	}
	r.LossRate = rate(r.Losses, r.Battles)

	var all []CardThreat
	for card, n := range faced {
		roles, _ := RolesFor(card)
		if roles == nil {
			roles = []Role{}
		}
		all = append(all, CardThreat{
			Card:         card,
			Roles:        roles,
			Faced:        n,
			Losses:       lost[card],
			LossRate:     rate(lost[card], n),
			ExcessLosses: roundTo3(float64(lost[card]) - float64(n)*r.LossRate),
		})
	}
	// Sort by name first so ties come out in a stable order.
	slices.SortFunc(all, func(x, y CardThreat) int { return cmp.Compare(x.Card, y.Card) })

	for _, t := range all {
		if slices.Contains(t.Roles, WinCondition) {
			r.WinConditions = append(r.WinConditions, t)
		}
		if t.Faced >= minFacedForThreat && t.ExcessLosses > 0 {
			r.Threats = append(r.Threats, t)
		}
	}
	slices.SortStableFunc(r.Threats, func(x, y CardThreat) int { return cmp.Compare(y.ExcessLosses, x.ExcessLosses) })
	r.Threats = r.Threats[:min(maxThreats, len(r.Threats))]
	slices.SortStableFunc(r.WinConditions, func(x, y CardThreat) int {
		return cmp.Or(cmp.Compare(y.LossRate, x.LossRate), cmp.Compare(y.Faced, x.Faced))
	})

	switch {
	case r.Battles == 0 && deck != nil:
		r.Notes = append(r.Notes, "no standard 1v1 battles with this deck in the log; try all decks")
	case r.Battles == 0:
		r.Notes = append(r.Notes, "no standard 1v1 battles in the log")
	case r.Battles < 10:
		r.Notes = append(r.Notes, fmt.Sprintf("only %d battles analyzed; treat these as hints, not conclusions", r.Battles))
	}
	return r
}
