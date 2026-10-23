package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"clash-analyzer/internal/clashapi"
	"clash-analyzer/internal/deck"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type getPlayerInput struct {
	Tag string `json:"tag" jsonschema:"the player's tag, e.g. #U9LGVRQQU (the leading # is optional)"`
}

func main() {
	// stdout carries the MCP protocol, so all diagnostics go to stderr.
	log.SetOutput(os.Stderr)

	// Load .env if present. Variables already set in the environment win.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalln("loading .env:", err)
	}

	token := os.Getenv("CLASH_API_TOKEN")
	if token == "" {
		log.Fatalln("CLASH_API_TOKEN is not set (see .env.example)")
	}
	client := clashapi.NewClient(token)

	server := mcp.NewServer(&mcp.Implementation{Name: "clash-analyzer", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_player",
		Description: "Look up a Clash Royale player by tag. Returns their name, every unlocked card with levels, and their current battle deck.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getPlayerInput) (*mcp.CallToolResult, clashapi.Player, error) {
		player, err := client.GetPlayer(ctx, in.Tag)
		if err != nil {
			return nil, clashapi.Player{}, fmt.Errorf("getting player %q: %w", in.Tag, err)
		}
		return nil, player, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_deck",
		Description: "Analyze a player's current battle deck: average elixir, cheapest 4-card cycle, card roles (win condition, spells, buildings, air defense), and gaps such as no air defense or no small spell.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getPlayerInput) (*mcp.CallToolResult, deck.Analysis, error) {
		player, err := client.GetPlayer(ctx, in.Tag)
		if err != nil {
			return nil, deck.Analysis{}, fmt.Errorf("getting player %q: %w", in.Tag, err)
		}
		return nil, deck.Analyze(player.CurrentDeck), nil
	})

	registerBattleTools(server, client)
	registerCardTools(server, client)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalln(err)
	}
}
