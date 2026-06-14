// Package nominatim is the library behind the nominatim command line:
// the HTTP client, request shaping, and the typed data models for the
// Nominatim geocoding service (nominatim.openstreetmap.org).
//
// Usage policy: every request must carry a meaningful User-Agent header
// identifying the application and a contact URL or email. The service
// enforces a hard limit of 1 request per second; the Client paces at
// 1100 ms to stay safely within the limit.
package nominatim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "nominatim.openstreetmap.org"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults: a 1100ms rate limit
// (10% above the 1 req/s policy limit), 15s timeout, and 3 retries.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://nominatim.openstreetmap.org",
		UserAgent: "nominatim-cli/dev (+https://github.com/tamnd/nominatim-cli)",
		Rate:      1100 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to Nominatim over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Search performs forward geocoding: converts a free-form query string into a
// list of matching locations. If limit <= 0 the caller's default is applied;
// pass a positive value to cap results (max 50 per the API).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Location, error) {
	n := limit
	if n <= 0 {
		n = 5
	}
	u := fmt.Sprintf("%s/search?q=%s&format=json&limit=%d&addressdetails=0",
		c.cfg.BaseURL, neturl.QueryEscape(query), n)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []rawLocation
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	items := make([]Location, 0, len(raw))
	for i, r := range raw {
		loc, err := r.toLocation(i + 1)
		if err != nil {
			return nil, err
		}
		items = append(items, loc)
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

// Lookup looks up one or more OSM objects by their prefixed IDs.
// IDs must carry the type prefix: N=node, R=relation, W=way (e.g. "R7444").
// The Nominatim parameter is named osmnodes regardless of the ID type.
func (c *Client) Lookup(ctx context.Context, osmIDs []string) ([]Location, error) {
	if len(osmIDs) == 0 {
		return nil, fmt.Errorf("lookup: no OSM IDs provided")
	}
	joined := strings.Join(osmIDs, ",")
	u := fmt.Sprintf("%s/lookup?osmnodes=%s&format=json&addressdetails=0",
		c.cfg.BaseURL, neturl.QueryEscape(joined))
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []rawLocation
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode lookup response: %w", err)
	}
	items := make([]Location, 0, len(raw))
	for i, r := range raw {
		loc, err := r.toLocation(i + 1)
		if err != nil {
			return nil, err
		}
		items = append(items, loc)
	}
	return items, nil
}

// Reverse performs reverse geocoding: converts a latitude/longitude pair into a
// structured address.
func (c *Client) Reverse(ctx context.Context, lat, lon float64) (*Address, error) {
	u := fmt.Sprintf("%s/reverse?lat=%.7f&lon=%.7f&format=json&addressdetails=1",
		c.cfg.BaseURL, lat, lon)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw rawReverse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode reverse response: %w", err)
	}
	return raw.toAddress()
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

// osmURL constructs the canonical OpenStreetMap URL for a given osm_type and
// osm_id. The osm_type from /search is already lowercase ("relation", "node",
// "way"); single-letter codes (R/W/N) from other endpoints are expanded too.
func osmURL(osmType string, osmID int64) string {
	t := strings.ToLower(osmType)
	switch t {
	case "r":
		t = "relation"
	case "w":
		t = "way"
	case "n":
		t = "node"
	}
	return "https://www.openstreetmap.org/" + t + "/" + strconv.FormatInt(osmID, 10)
}
