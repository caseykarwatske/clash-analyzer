package main

import (
	"context"
	"fmt"
	"sync"

	"clash-analyzer/internal/clashapi"
	"clash-analyzer/internal/deck"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTopPlayers = 10
	maxTopPlayers     = 50
	fetchConcurrency  = 8
	// countersPerCard caps how many counters analyze_meta lists per card.
	countersPerCard = 5
	metaListSize    = 15
)

type battleLogInput struct {
	Tag   string `json:"tag" jsonschema:"the player's tag, e.g. #U9LGVRQQU (the leading # is optional)"`
	Limit int    `json:"limit,omitempty" jsonschema:"max battles to return, newest first (default: all, up to ~30)"`
}

type battleLogOutput struct {
	Tag     string               `json:"tag"`
	Battles []deck.BattleSummary `json:"battles"`
}

type topPlayersInput struct {
	Limit            int  `json:"limit,omitempty" jsonschema:"number of top Path of Legend players (default 10, max 50)"`
	IncludeBattles   bool `json:"includeBattles,omitempty" jsonschema:"also return each player's recent battles"`
	BattlesPerPlayer int  `json:"battlesPerPlayer,omitempty" jsonschema:"max battles per player when includeBattles is set (default 10)"`
}

type topPlayer struct {
	Rank          int                  `json:"rank"`
	Tag           string               `json:"tag"`
	Name          string               `json:"name"`
	EloRating     int                  `json:"eloRating"`
	Clan          string               `json:"clan,omitempty"`
	Deck          []string             `json:"deck"`
	AverageElixir float64              `json:"averageElixir"`
	Battles       []deck.BattleSummary `json:"battles,omitempty"`
	Error         string               `json:"error,omitempty" jsonschema:"set if this player's data couldn't be fetched"`
}

type topPlayersOutput struct {
	Players []topPlayer `json:"players"`
}

type lossesInput struct {
	Tag      string `json:"tag" jsonschema:"the player's tag, e.g. #U9LGVRQQU (the leading # is optional)"`
	AllDecks bool   `json:"allDecks,omitempty" jsonschema:"count battles with any deck, not just the player's current deck"`
}

type lossesOutput struct {
	Deck       []string            `json:"deck"`
	DeckGaps   []string            `json:"deckGaps" jsonschema:"structural gaps in the current deck, e.g. no air defense"`
	Weaknesses deck.WeaknessReport `json:"weaknesses"`
}

type metaInput struct {
	TopPlayers int    `json:"topPlayers,omitempty" jsonschema:"how many top Path of Legend players' battle logs to sample (default 10, max 50)"`
	MinGames   int    `json:"minGames,omitempty" jsonschema:"minimum games for a card or counter to be ranked by win rate (default 5)"`
	Tag        string `json:"tag,omitempty" jsonschema:"optional player tag; if set, counters are listed for each card in that player's current deck"`
}

type metaOutput struct {
	PlayersSampled int                 `json:"playersSampled"`
	Meta           deck.Meta           `json:"meta"`
	CountersFor    []string            `json:"countersFor" jsonschema:"the cards counters were computed for: the given player's deck, or else the most popular cards"`
	Counters       []deck.CardCounters `json:"counters"`
}

func registerBattleTools(server *mcp.Server, client *clashapi.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_battle_log",
		Description: "Get a Clash Royale player's recent battles (up to ~30), newest first: mode, result, crowns, both decks and the opponent.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in battleLogInput) (*mcp.CallToolResult, battleLogOutput, error) {
		battles, err := client.GetBattleLog(ctx, in.Tag)
		if err != nil {
			return nil, battleLogOutput{}, fmt.Errorf("getting battle log for %q: %w", in.Tag, err)
		}
		return nil, battleLogOutput{Tag: clashapi.NormalizeTag(in.Tag), Battles: summarize(battles, in.Tag, in.Limit)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_top_players",
		Description: "Get the top players on the global Path of Legend leaderboard with their current decks, and optionally their recent battles.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in topPlayersInput) (*mcp.CallToolResult, topPlayersOutput, error) {
		top, err := fetchTopPlayers(ctx, client, in.Limit, true, in.IncludeBattles)
		if err != nil {
			return nil, topPlayersOutput{}, err
		}
		perPlayer := in.BattlesPerPlayer
		if perPlayer <= 0 {
			perPlayer = 10
		}
		out := topPlayersOutput{Players: make([]topPlayer, len(top))}
		for i, t := range top {
			p := topPlayer{
				Rank:      t.ranked.Rank,
				Tag:       t.ranked.Tag,
				Name:      t.ranked.Name,
				EloRating: t.ranked.EloRating,
				Deck:      []string{},
			}
			if t.ranked.Clan != nil {
				p.Clan = t.ranked.Clan.Name
			}
			if t.err != nil {
				p.Error = t.err.Error()
			}
			for _, c := range t.player.CurrentDeck {
				p.Deck = append(p.Deck, c.Name)
			}
			p.AverageElixir = deck.Analyze(t.player.CurrentDeck).AverageElixir
			if in.IncludeBattles {
				p.Battles = summarize(t.battles, t.ranked.Tag, perPlayer)
			}
			out.Players[i] = p
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_losses",
		Description: "Find weaknesses in a player's current deck from their recent standard 1v1 battles: which opponent cards and win conditions they lose to most, plus structural gaps in the deck.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in lossesInput) (*mcp.CallToolResult, lossesOutput, error) {
		player, err := client.GetPlayer(ctx, in.Tag)
		if err != nil {
			return nil, lossesOutput{}, fmt.Errorf("getting player %q: %w", in.Tag, err)
		}
		battles, err := client.GetBattleLog(ctx, in.Tag)
		if err != nil {
			return nil, lossesOutput{}, fmt.Errorf("getting battle log for %q: %w", in.Tag, err)
		}

		analysis := deck.Analyze(player.CurrentDeck)
		out := lossesOutput{Deck: []string{}, DeckGaps: analysis.Gaps}
		for _, c := range player.CurrentDeck {
			out.Deck = append(out.Deck, c.Name)
		}
		var filter []string
		if !in.AllDecks {
			filter = out.Deck
		}
		out.Weaknesses = deck.AnalyzeLosses(battles, in.Tag, filter)
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_meta",
		Description: "Sample top Path of Legend players' battle logs to find the most played and strongest cards and decks, and which cards counter which. Pass a player tag to get counters for each card in their deck.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in metaInput) (*mcp.CallToolResult, metaOutput, error) {
		minGames := in.MinGames
		if minGames <= 0 {
			minGames = 5
		}

		var target []string
		if in.Tag != "" {
			player, err := client.GetPlayer(ctx, in.Tag)
			if err != nil {
				return nil, metaOutput{}, fmt.Errorf("getting player %q: %w", in.Tag, err)
			}
			for _, c := range player.CurrentDeck {
				target = append(target, c.Name)
			}
		}

		top, err := fetchTopPlayers(ctx, client, in.TopPlayers, false, true)
		if err != nil {
			return nil, metaOutput{}, err
		}
		var battles []clashapi.Battle
		sampled := 0
		for _, t := range top {
			if t.err == nil {
				sampled++
				battles = append(battles, t.battles...)
			}
		}
		if sampled == 0 && len(top) > 0 {
			return nil, metaOutput{}, fmt.Errorf("fetching top player battle logs: %w", top[0].err)
		}

		out := metaOutput{PlayersSampled: sampled, Meta: deck.MetaFromBattles(battles, minGames, metaListSize)}
		if target == nil {
			// No player given: show counters to the 8 most popular cards.
			for _, c := range out.Meta.Popular[:min(8, len(out.Meta.Popular))] {
				target = append(target, c.Card)
			}
		}
		out.CountersFor = append([]string{}, target...)
		out.Counters = deck.Counters(battles, target, minGames, countersPerCard)
		return nil, out, nil
	})
}

func summarize(battles []clashapi.Battle, tag string, limit int) []deck.BattleSummary {
	if limit > 0 && limit < len(battles) {
		battles = battles[:limit]
	}
	out := make([]deck.BattleSummary, len(battles))
	for i, b := range battles {
		out[i] = deck.Summarize(b, tag)
	}
	return out
}

type topPlayerData struct {
	ranked  clashapi.RankedPlayer
	player  clashapi.Player
	battles []clashapi.Battle
	err     error // first error fetching this player's data, if any
}

// fetchTopPlayers gets the leaderboard and then, concurrently, each player's
// profile and/or battle log. Per-player failures are recorded in err rather
// than failing the whole call.
func fetchTopPlayers(ctx context.Context, client *clashapi.Client, limit int, withProfile, withBattles bool) ([]topPlayerData, error) {
	if limit <= 0 {
		limit = defaultTopPlayers
	}
	limit = min(limit, maxTopPlayers)

	ranked, err := client.GetTopPlayers(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("getting top players: %w", err)
	}

	out := make([]topPlayerData, len(ranked))
	sem := make(chan struct{}, fetchConcurrency)
	var wg sync.WaitGroup
	for i, r := range ranked {
		out[i].ranked = r
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			d := &out[i]
			var err error
			if withProfile {
				if d.player, err = client.GetPlayer(ctx, r.Tag); err != nil {
					d.err = fmt.Errorf("getting player %s: %w", r.Tag, err)
					return
				}
			}
			if withBattles {
				if d.battles, err = client.GetBattleLog(ctx, r.Tag); err != nil {
					d.err = fmt.Errorf("getting battle log for %s: %w", r.Tag, err)
				}
			}
		})
	}
	wg.Wait()
	return out, nil
}
