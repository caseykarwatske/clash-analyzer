package clashapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://proxy.royaleapi.dev/v1"

// Client calls the Clash Royale API (via the RoyaleAPI proxy).
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient returns a Client that authenticates with the given API token.
func NewClient(token string) *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// APIError is returned when the API responds with a non-200 status.
type APIError struct {
	StatusCode int
	Reason     string `json:"reason"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("clash api: %d %s: %s", e.StatusCode, e.Reason, e.Message)
}

// NormalizeTag uppercases a player tag and ensures it has a leading '#'.
func NormalizeTag(tag string) string {
	return "#" + strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
}

func playerPath(tag string) (string, error) {
	tag = NormalizeTag(tag)
	if tag == "#" {
		return "", errors.New("clash api: empty player tag")
	}
	return "/players/" + url.PathEscape(tag), nil
}

// GetPlayer fetches a player by tag. The leading '#' is optional.
func (c *Client) GetPlayer(ctx context.Context, tag string) (Player, error) {
	var player Player
	path, err := playerPath(tag)
	if err != nil {
		return player, err
	}
	err = c.get(ctx, path, &player)
	return player, err
}

// GetBattleLog fetches a player's recent battles (up to ~30), newest first.
func (c *Client) GetBattleLog(ctx context.Context, tag string) ([]Battle, error) {
	var battles []Battle
	path, err := playerPath(tag)
	if err != nil {
		return nil, err
	}
	err = c.get(ctx, path+"/battlelog", &battles)
	return battles, err
}

// GetCards fetches every card in the game. Level fields are unset.
func (c *Client) GetCards(ctx context.Context) ([]Card, error) {
	var resp struct {
		Items []Card `json:"items"`
	}
	err := c.get(ctx, "/cards", &resp)
	return resp.Items, err
}

// GetTopPlayers fetches the top of the current global Path of Legend leaderboard.
func (c *Client) GetTopPlayers(ctx context.Context, limit int) ([]RankedPlayer, error) {
	var resp struct {
		Items []RankedPlayer `json:"items"`
	}
	err := c.get(ctx, "/locations/global/pathoflegend/players?limit="+strconv.Itoa(limit), &resp)
	return resp.Items, err
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = json.Unmarshal(body, apiErr)
		return apiErr
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("clash api: decoding response: %w", err)
	}
	return nil
}
