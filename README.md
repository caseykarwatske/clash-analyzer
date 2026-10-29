# Clash Analyzer

**Clash Royale analytics through the Model Context Protocol (MCP).**

Clash Analyzer is a Go-based MCP server that connects AI assistants to Clash Royale player data and provides tools for investigating decks, recent battles, card collections, and the competitive meta. It fetches data from the Clash Royale API and performs analysis in Go, returning structured results that an MCP-compatible client can use in conversation.

## Features

### Player and deck analysis
- Look up a player by tag to retrieve their profile, unlocked cards, and current battle deck.
- Analyze a deck's average elixir, cheapest four-card cycle, card roles (such as win conditions, spells, buildings, and air defense), and structural gaps.
- Inspect unlocked and locked cards, with filters for rarity and minimum level, including evolution information and counts by rarity.

### Battle and meta analysis
- Retrieve recent battles, including match results, modes, crowns, player and opponent decks.
- Analyze losses against opponent cards and win conditions, optionally restricting results to matches using the player's current deck.
- Inspect top players on the global Path of Legend leaderboard, with optional recent battle histories.
- Sample top-player battle logs to analyze card and deck popularity, estimated win rates, and card counters. Results depend on the available sample, not all battles played globally.

### Backend
- Exposes seven read-only MCP tools over **stdio**.
- Uses an authenticated Clash Royale API client with request timeouts and structured HTTP error handling.
- Fetches leaderboard player data concurrently, bounded to eight concurrent workers, and preserves per-player errors instead of failing the entire batch.

## Available MCP tools

| Tool | Description |
| --- | --- |
| `get_player` | Retrieve a player profile, cards, and current deck by player tag. |
| `analyze_deck` | Calculate deck metrics, card roles, and structural weaknesses. |
| `get_battle_log` | Retrieve a player's recent battle summaries. |
| `get_top_players` | Retrieve leaderboard players, decks, and optional battle logs. |
| `analyze_losses` | Identify matchup patterns and structural gaps in a player's deck. |
| `analyze_meta` | Analyze popular cards/decks, win rates, and counters from sampled top-player battles. |
| `get_unlocked_cards` | Inspect a player's card collection with rarity and level filters. |

## Getting started

### Prerequisites
- Go **1.27.2** (the version declared in `go.mod`)
- A Clash Royale API token from [the developer portal](https://developer.clashroyale.com/)
- An MCP-compatible client that can launch a local stdio server

The API client uses the RoyaleAPI proxy at `https://proxy.royaleapi.dev/v1`. When configuring your API token, follow the IP allowlisting note in `.env.example`.

### Install and configure

```bash
git clone https://github.com/caseykarwatske/clash-analyzer.git
cd clash-analyzer
cp .env.example .env
```

Set your token in `.env`:

```dotenv
CLASH_API_TOKEN=your_token_here
```

### Run

```bash
go run ./cmd/server
```

The server communicates via **standard input and output** using MCP; it is not a conventional HTTP server or an interactive terminal application. Logs are sent to stderr.

### Connect an MCP client

Configure your MCP-compatible client to launch the server from the project directory, for example:

```json
{
  "mcpServers": {
    "clash-analyzer": {
      "command": "go",
      "args": ["run", "./cmd/server"],
      "cwd": "/absolute/path/to/clash-analyzer"
    }
  }
}
```

> MCP configuration formats differ by client. The example shows the command, arguments, and working directory needed for a client that supports this configuration format. The server reads its API token from `.env` or the process environment.

## Example questions

Once connected to an MCP-compatible AI assistant, try:

- “Analyze the deck for player `#YOURTAG`. What are its weaknesses?”
- “Which cards have appeared most often in the sampled top-player battles?”
- “What opponent win conditions have I lost to recently?”
- “Show my unlocked legendary cards and their levels.”
- “Compare popular cards and their sampled win rates.”

The assistant chooses the relevant MCP tool(s); the Go server retrieves and analyzes the underlying data.

## Architecture

```text
MCP-compatible AI client
          |
          | MCP (stdio)
          v
   Go MCP server
          |
    +-----+------------------+
    |                        |
Clash Royale API       Deck / battle analytics
client (HTTP)           (Go)
    |                        ^
    +------ player, card, ---+
            battle and
            leaderboard data
```

The server entry point lives in `cmd/server`. Domain logic and the API client are organized under `internal/`.

## How to interpret the results

Deck role classifications and gap detection use rule-based analysis. Meta statistics and counters are estimates derived from sampled recent battles of highly ranked players, and may be sensitive to sample size, repeated encounters, or changes in the game meta. They should not be interpreted as definitive global win rates or guaranteed matchup outcomes.

## Technology

**Go · Model Context Protocol (MCP) · Clash Royale REST API · JSON · Concurrent HTTP requests**

## Disclaimer

This is an independent community project and is not affiliated with, endorsed by, or sponsored by Supercell. Clash Royale and related marks belong to their respective owners.
