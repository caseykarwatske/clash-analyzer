package clashapi

// Player is a subset of the response from GET /v1/players/{playerTag}.
// Fields not listed here are ignored when decoding.
type Player struct {
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Cards       []Card `json:"cards"`       // every card the player has unlocked
	CurrentDeck []Card `json:"currentDeck"` // the player's active battle deck
}

type Card struct {
	ID                int      `json:"id"`
	Name              string   `json:"name"`
	Rarity            string   `json:"rarity"`   // common, rare, epic, legendary, champion
	Level             int      `json:"level"`    // relative to rarity; see DisplayLevel
	MaxLevel          int      `json:"maxLevel"` // also relative: 16 common, 14 rare, 11 epic, 8 legendary, 6 champion
	StarLevel         int      `json:"starLevel,omitempty"`
	EvolutionLevel    int      `json:"evolutionLevel,omitempty"`
	MaxEvolutionLevel int      `json:"maxEvolutionLevel,omitempty"`
	ElixirCost        int      `json:"elixirCost,omitempty"` // absent for Mirror
	Count             int      `json:"count"`
	IconURLs          IconURLs `json:"iconUrls"`
}

type IconURLs struct {
	Medium          string `json:"medium"`
	EvolutionMedium string `json:"evolutionMedium,omitempty"`
}

// Battle is one entry from GET /v1/players/{playerTag}/battlelog.
// Team is the player whose log was requested (plus a teammate in 2v2).
type Battle struct {
	Type          string         `json:"type"`          // PvP (ladder), pathOfLegend, riverRacePvP, friendly, ...
	BattleTime    string         `json:"battleTime"`    // e.g. 20261022T234530.000Z
	DeckSelection string         `json:"deckSelection"` // collection, draft, eventDeck, warDeckPick, ...
	GameMode      GameMode       `json:"gameMode"`
	Arena         Arena          `json:"arena"`
	Team          []BattlePlayer `json:"team"`
	Opponent      []BattlePlayer `json:"opponent"`
}

type BattlePlayer struct {
	Tag              string  `json:"tag"`
	Name             string  `json:"name"`
	Crowns           int     `json:"crowns"`
	StartingTrophies int     `json:"startingTrophies,omitempty"`
	TrophyChange     int     `json:"trophyChange,omitempty"`
	ElixirLeaked     float64 `json:"elixirLeaked,omitempty"`
	Cards            []Card  `json:"cards"`
}

type GameMode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Arena struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RankedPlayer is an entry from the Path of Legend leaderboard.
type RankedPlayer struct {
	Tag       string `json:"tag"`
	Name      string `json:"name"`
	ExpLevel  int    `json:"expLevel"`
	EloRating int    `json:"eloRating"`
	Rank      int    `json:"rank"`
	Clan      *Clan  `json:"clan,omitempty"`
}

type Clan struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
}

// MaxDisplayLevel is the in-game max level, the same for every rarity.
const MaxDisplayLevel = 16

// DisplayLevel converts the API's rarity-relative level to the level shown
// in game (e.g. a legendary at API level 8 of 8 is level 16).
func (c Card) DisplayLevel() int {
	if c.MaxLevel == 0 {
		return c.Level
	}
	return c.Level + MaxDisplayLevel - c.MaxLevel
}
