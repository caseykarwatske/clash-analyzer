package deck

import (
	"math"
	"slices"
	"strings"

	"clash-analyzer/internal/clashapi"
)

type Result string

const (
	Win  Result = "win"
	Loss Result = "loss"
	Draw Result = "draw"
)

// BattleSummary is a compact, single-perspective view of a battle.
type BattleSummary struct {
	Time           string   `json:"time"`
	Type           string   `json:"type"`
	GameMode       string   `json:"gameMode"`
	Result         Result   `json:"result"`
	Crowns         int      `json:"crowns"`
	OpponentCrowns int      `json:"opponentCrowns"`
	TrophyChange   int      `json:"trophyChange,omitempty"`
	Deck           []string `json:"deck"`
	Opponent       string   `json:"opponent"`
	OpponentTag    string   `json:"opponentTag"`
	OpponentDeck   []string `json:"opponentDeck"`
}

// Summarize describes a battle from the side of the player with the given
// tag. If the tag isn't in the battle, the "team" side is used.
func Summarize(b clashapi.Battle, tag string) BattleSummary {
	me, them := sides(b, tag)
	s := BattleSummary{
		Time:         b.BattleTime,
		Type:         b.Type,
		GameMode:     b.GameMode.Name,
		Deck:         []string{},
		OpponentDeck: []string{},
	}
	if len(me) > 0 && len(them) > 0 {
		s.Crowns, s.OpponentCrowns = me[0].Crowns, them[0].Crowns
		s.TrophyChange = me[0].TrophyChange
		s.Opponent, s.OpponentTag = them[0].Name, them[0].Tag
	}
	s.Result = result(s.Crowns, s.OpponentCrowns)
	for _, p := range me {
		s.Deck = append(s.Deck, cardNames(p.Cards)...)
	}
	for _, p := range them {
		s.OpponentDeck = append(s.OpponentDeck, cardNames(p.Cards)...)
	}
	return s
}

// competitiveTypes are battle types played with a player's own deck under
// normal rules. Friendlies, events and draft modes are left out.
var competitiveTypes = []string{"PvP", "pathOfLegend", "riverRacePvP"}

// IsStandard1v1 reports whether a battle is a competitive 1v1 played with
// each player's own 8-card deck, so its results say something about decks.
func IsStandard1v1(b clashapi.Battle) bool {
	return slices.Contains(competitiveTypes, b.Type) &&
		b.DeckSelection == "collection" &&
		len(b.Team) == 1 && len(b.Opponent) == 1 &&
		len(b.Team[0].Cards) == 8 && len(b.Opponent[0].Cards) == 8
}

// sides returns (player's side, opposing side) for the given tag.
func sides(b clashapi.Battle, tag string) (me, them []clashapi.BattlePlayer) {
	tag = clashapi.NormalizeTag(tag)
	for _, p := range b.Opponent {
		if clashapi.NormalizeTag(p.Tag) == tag {
			return b.Opponent, b.Team
		}
	}
	return b.Team, b.Opponent
}

func result(crowns, opponentCrowns int) Result {
	switch {
	case crowns > opponentCrowns:
		return Win
	case crowns < opponentCrowns:
		return Loss
	default:
		return Draw
	}
}

func cardNames(cards []clashapi.Card) []string {
	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = c.Name
	}
	return names
}

// SameDeck reports whether two card lists hold the same cards in any order.
func SameDeck(a, b []string) bool {
	return deckKey(a) == deckKey(b)
}

// deckKey is an order-independent identifier for a deck.
func deckKey(cards []string) string {
	sorted := slices.Clone(cards)
	slices.Sort(sorted)
	return strings.Join(sorted, "|")
}

func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return roundTo3(float64(n) / float64(d))
}

func roundTo3(x float64) float64 {
	return math.Round(x*1000) / 1000
}
