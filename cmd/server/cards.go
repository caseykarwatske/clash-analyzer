package main

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"clash-analyzer/internal/clashapi"
	"clash-analyzer/internal/deck"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var rarityOrder = []string{"common", "rare", "epic", "legendary", "champion"}

type cardsInput struct {
	Tag      string `json:"tag" jsonschema:"the player's tag, e.g. #U9LGVRQQU (the leading # is optional)"`
	Rarity   string `json:"rarity,omitempty" jsonschema:"only cards of this rarity: common, rare, epic, legendary or champion"`
	MinLevel int    `json:"minLevel,omitempty" jsonschema:"only cards at or above this in-game level (1-16)"`
}

type unlockedCard struct {
	Name         string      `json:"name"`
	Rarity       string      `json:"rarity"`
	Elixir       int         `json:"elixir"` // 0 for Mirror
	Level        int         `json:"level" jsonschema:"in-game level (max 16 for every rarity)"`
	StarLevel    int         `json:"starLevel,omitempty"`
	Evolution    int         `json:"evolution,omitempty" jsonschema:"evolution level unlocked, if any"`
	MaxEvolution int         `json:"maxEvolution,omitempty"`
	Count        int         `json:"count" jsonschema:"spare copies held toward the next upgrade"`
	Roles        []deck.Role `json:"roles"`
}

type rarityCount struct {
	Rarity   string `json:"rarity"`
	Unlocked int    `json:"unlocked"`
	Total    int    `json:"total"`
	Maxed    int    `json:"maxed" jsonschema:"unlocked cards at level 16"`
}

type cardsOutput struct {
	Tag      string         `json:"tag"`
	Unlocked int            `json:"unlocked"`
	Total    int            `json:"total" jsonschema:"cards in the game"`
	ByRarity []rarityCount  `json:"byRarity"`
	Cards    []unlockedCard `json:"cards" jsonschema:"unlocked cards matching the filters, highest level first"`
	Locked   []string       `json:"locked" jsonschema:"cards the player hasn't unlocked yet (matching the rarity filter)"`
}

func registerCardTools(server *mcp.Server, client *clashapi.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_unlocked_cards",
		Description: "List the cards a Clash Royale player has unlocked, with in-game level, star level, evolution, elixir and role, plus counts per rarity and the cards still locked.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in cardsInput) (*mcp.CallToolResult, cardsOutput, error) {
		rarity := strings.ToLower(strings.TrimSpace(in.Rarity))
		if rarity != "" && !slices.Contains(rarityOrder, rarity) {
			return nil, cardsOutput{}, fmt.Errorf("unknown rarity %q (want one of %s)", in.Rarity, strings.Join(rarityOrder, ", "))
		}

		player, err := client.GetPlayer(ctx, in.Tag)
		if err != nil {
			return nil, cardsOutput{}, fmt.Errorf("getting player %q: %w", in.Tag, err)
		}
		all, err := client.GetCards(ctx)
		if err != nil {
			return nil, cardsOutput{}, fmt.Errorf("getting card list: %w", err)
		}
		return nil, unlockedCards(player, all, rarity, in.MinLevel), nil
	})
}

func unlockedCards(player clashapi.Player, all []clashapi.Card, rarity string, minLevel int) cardsOutput {
	out := cardsOutput{
		Tag:      player.Tag,
		Unlocked: len(player.Cards),
		Total:    len(all),
		Cards:    []unlockedCard{},
		Locked:   []string{},
	}

	counts := map[string]*rarityCount{}
	for _, r := range rarityOrder {
		counts[r] = &rarityCount{Rarity: r}
	}
	countFor := func(r string) *rarityCount {
		if counts[r] == nil {
			counts[r] = &rarityCount{Rarity: r}
		}
		return counts[r]
	}
	for _, c := range all {
		countFor(c.Rarity).Total++
	}

	owned := map[string]bool{}
	for _, c := range player.Cards {
		owned[c.Name] = true
		rc := countFor(c.Rarity)
		rc.Unlocked++
		if c.DisplayLevel() >= clashapi.MaxDisplayLevel {
			rc.Maxed++
		}

		if (rarity != "" && c.Rarity != rarity) || c.DisplayLevel() < minLevel {
			continue
		}
		roles, _ := deck.RolesFor(c.Name)
		if roles == nil {
			roles = []deck.Role{}
		}
		out.Cards = append(out.Cards, unlockedCard{
			Name:         c.Name,
			Rarity:       c.Rarity,
			Elixir:       c.ElixirCost,
			Level:        c.DisplayLevel(),
			StarLevel:    c.StarLevel,
			Evolution:    c.EvolutionLevel,
			MaxEvolution: c.MaxEvolutionLevel,
			Count:        c.Count,
			Roles:        roles,
		})
	}
	slices.SortFunc(out.Cards, func(x, y unlockedCard) int {
		return cmp.Or(cmp.Compare(y.Level, x.Level), cmp.Compare(x.Name, y.Name))
	})

	for _, c := range all {
		if !owned[c.Name] && (rarity == "" || c.Rarity == rarity) {
			out.Locked = append(out.Locked, c.Name)
		}
	}
	slices.Sort(out.Locked)

	// Known rarities in game order, then anything new the API adds.
	for _, r := range rarityOrder {
		out.ByRarity = append(out.ByRarity, *counts[r])
		delete(counts, r)
	}
	for _, r := range slices.Sorted(maps.Keys(counts)) {
		out.ByRarity = append(out.ByRarity, *counts[r])
	}
	return out
}
