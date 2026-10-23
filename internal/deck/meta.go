package deck

import (
	"cmp"
	"slices"

	"clash-analyzer/internal/clashapi"
)

// CardStat is how often a card is played and how often its deck wins.
type CardStat struct {
	Card    string  `json:"card"`
	Roles   []Role  `json:"roles"`
	Uses    int     `json:"uses" jsonschema:"decks (one per side per battle) containing this card"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"winRate"`
	UseRate float64 `json:"useRate" jsonschema:"share of decks containing this card"`
}

type DeckStat struct {
	Cards   []string `json:"cards"`
	Uses    int      `json:"uses"`
	Wins    int      `json:"wins"`
	WinRate float64  `json:"winRate"`
}

// Matchup is how decks with Card do against decks with some other card.
type Matchup struct {
	Card    string  `json:"card"`
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"winRate"`
}

type Meta struct {
	Battles   int        `json:"battles" jsonschema:"unique standard 1v1 battles analyzed"`
	Popular   []CardStat `json:"popular" jsonschema:"most-played cards"`
	Strongest []CardStat `json:"strongest" jsonschema:"highest win rate cards with at least minGames uses"`
	Decks     []DeckStat `json:"decks" jsonschema:"most-played exact decks"`
}

// CardCounters lists the opposing cards that do best against Card.
type CardCounters struct {
	Card string `json:"card"`
	// Games and WinRateAgainst are the baseline: how all opponents of
	// decks with Card fared. A good counter beats this number.
	Games          int       `json:"games"`
	WinRateAgainst float64   `json:"winRateAgainst" jsonschema:"overall win rate of decks facing this card"`
	Counters       []Matchup `json:"counters" jsonschema:"opposing cards with the highest win rate against this card"`
}

// side is one player's half of a standard 1v1 battle.
type side struct {
	cards    []string
	won      bool
	lost     bool
	opponent []string
}

// standardSides dedupes battles (two top players facing each other appear in
// both logs) and returns both sides of every standard 1v1 battle.
func standardSides(battles []clashapi.Battle) (sides []side, n int) {
	seen := map[string]bool{}
	for _, b := range battles {
		if !IsStandard1v1(b) {
			continue
		}
		t, o := b.Team[0], b.Opponent[0]
		key := b.BattleTime + deckKey([]string{clashapi.NormalizeTag(t.Tag), clashapi.NormalizeTag(o.Tag)})
		if seen[key] {
			continue
		}
		seen[key] = true
		n++

		tc, oc := cardNames(t.Cards), cardNames(o.Cards)
		sides = append(sides,
			side{cards: tc, won: t.Crowns > o.Crowns, lost: t.Crowns < o.Crowns, opponent: oc},
			side{cards: oc, won: o.Crowns > t.Crowns, lost: o.Crowns < t.Crowns, opponent: tc},
		)
	}
	return sides, n
}

// MetaFromBattles finds the most played and strongest cards and decks. Cards
// and decks need minGames uses to count as strongest; each list is capped
// at top entries.
func MetaFromBattles(battles []clashapi.Battle, minGames, top int) Meta {
	sides, n := standardSides(battles)
	m := Meta{Battles: n, Popular: []CardStat{}, Strongest: []CardStat{}, Decks: []DeckStat{}}

	cards := map[string]*CardStat{}
	decks := map[string]*DeckStat{}
	for _, s := range sides {
		for _, c := range s.cards {
			st := cards[c]
			if st == nil {
				roles, _ := RolesFor(c)
				if roles == nil {
					roles = []Role{}
				}
				st = &CardStat{Card: c, Roles: roles}
				cards[c] = st
			}
			st.Uses++
			if s.won {
				st.Wins++
			}
		}
		k := deckKey(s.cards)
		d := decks[k]
		if d == nil {
			sorted := slices.Clone(s.cards)
			slices.Sort(sorted)
			d = &DeckStat{Cards: sorted}
			decks[k] = d
		}
		d.Uses++
		if s.won {
			d.Wins++
		}
	}

	var all []CardStat
	for _, st := range cards {
		st.WinRate = rate(st.Wins, st.Uses)
		st.UseRate = rate(st.Uses, len(sides))
		all = append(all, *st)
	}
	slices.SortFunc(all, func(x, y CardStat) int {
		return cmp.Or(cmp.Compare(y.Uses, x.Uses), cmp.Compare(x.Card, y.Card))
	})
	m.Popular = append(m.Popular, all[:min(top, len(all))]...)

	for _, st := range all {
		if st.Uses >= minGames {
			m.Strongest = append(m.Strongest, st)
		}
	}
	slices.SortStableFunc(m.Strongest, func(x, y CardStat) int { return cmp.Compare(y.WinRate, x.WinRate) })
	m.Strongest = m.Strongest[:min(top, len(m.Strongest))]

	for _, d := range decks {
		d.WinRate = rate(d.Wins, d.Uses)
		m.Decks = append(m.Decks, *d)
	}
	slices.SortFunc(m.Decks, func(x, y DeckStat) int {
		return cmp.Or(cmp.Compare(y.Uses, x.Uses), cmp.Compare(y.WinRate, x.WinRate), cmp.Compare(deckKey(x.Cards), deckKey(y.Cards)))
	})
	m.Decks = m.Decks[:min(top, len(m.Decks))]
	return m
}

// Counters finds, for each card, the opposing cards whose decks win most
// often against decks containing it. Battles where both sides run the card
// are skipped, as are counter cards the card's own side also runs. A
// counter needs at least minGames games; at most top are returned per card.
func Counters(battles []clashapi.Battle, cards []string, minGames, top int) []CardCounters {
	sides, _ := standardSides(battles)
	out := make([]CardCounters, 0, len(cards))
	for _, card := range cards {
		cc := CardCounters{Card: card, Counters: []Matchup{}}
		games := map[string]int{}
		wins := map[string]int{}
		oppWins := 0
		for _, s := range sides {
			if !slices.Contains(s.cards, card) || slices.Contains(s.opponent, card) {
				continue
			}
			cc.Games++
			if s.lost {
				oppWins++
			}
			for _, o := range s.opponent {
				if slices.Contains(s.cards, o) {
					continue
				}
				games[o]++
				if s.lost {
					wins[o]++
				}
			}
		}
		cc.WinRateAgainst = rate(oppWins, cc.Games)

		for o, g := range games {
			if g >= minGames {
				cc.Counters = append(cc.Counters, Matchup{Card: o, Games: g, Wins: wins[o], WinRate: rate(wins[o], g)})
			}
		}
		slices.SortFunc(cc.Counters, func(x, y Matchup) int {
			return cmp.Or(cmp.Compare(y.WinRate, x.WinRate), cmp.Compare(y.Games, x.Games), cmp.Compare(x.Card, y.Card))
		})
		cc.Counters = cc.Counters[:min(top, len(cc.Counters))]
		out = append(out, cc)
	}
	return out
}
