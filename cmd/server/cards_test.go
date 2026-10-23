package main

import (
	"slices"
	"testing"

	"clash-analyzer/internal/clashapi"
)

func TestUnlockedCards(t *testing.T) {
	all := []clashapi.Card{
		{Name: "Knight", Rarity: "common"},
		{Name: "Zap", Rarity: "common"},
		{Name: "The Log", Rarity: "legendary"},
		{Name: "Miner", Rarity: "legendary"},
	}
	player := clashapi.Player{Tag: "#ME", Cards: []clashapi.Card{
		{Name: "Knight", Rarity: "common", Level: 14, MaxLevel: 16},
		{Name: "The Log", Rarity: "legendary", Level: 8, MaxLevel: 8},
	}}

	out := unlockedCards(player, all, "", 0)
	if out.Unlocked != 2 || out.Total != 4 || !slices.Equal(out.Locked, []string{"Miner", "Zap"}) {
		t.Errorf("unlocked %d/%d, locked %v", out.Unlocked, out.Total, out.Locked)
	}
	// Highest in-game level first: a maxed legendary is level 16.
	if out.Cards[0].Name != "The Log" || out.Cards[0].Level != 16 || out.Cards[1].Level != 14 {
		t.Errorf("cards = %+v", out.Cards)
	}
	if c := out.ByRarity[3]; c.Rarity != "legendary" || c.Unlocked != 1 || c.Total != 2 || c.Maxed != 1 {
		t.Errorf("legendary count = %+v", c)
	}

	out = unlockedCards(player, all, "common", 15)
	if len(out.Cards) != 0 || !slices.Equal(out.Locked, []string{"Zap"}) {
		t.Errorf("filtered: cards %+v, locked %v", out.Cards, out.Locked)
	}
}
