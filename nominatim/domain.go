// Package nominatim exposes the Nominatim geocoding service as a kit Domain
// driver. A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/nominatim-cli/nominatim"
//
// The same Domain also builds the standalone nominatim binary (see cli/root.go),
// so the binary and a host share one source of truth.
package nominatim

import (
	"context"
	neturl "net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Nominatim driver. It carries no state; the per-run client is
// built by the factory Register hands to kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "nominatim",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "nominatim",
			Short:  "Nominatim geocoding (OpenStreetMap)",
			Long: `nominatim geocodes place names to coordinates and coordinates to addresses
using the public Nominatim API from OpenStreetMap. No API key required.

Usage policy: max 1 request/second. The client paces at 1100ms per request.`,
			Site: Host,
			Repo: "https://github.com/tamnd/nominatim-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// search: forward geocoding, place name -> coordinates
	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		List:    true,
		Summary: "Forward geocoding: convert a place name to coordinates",
		Args:    []kit.Arg{{Name: "query", Help: "place name or address to geocode"}},
	}, searchOp)

	// reverse: reverse geocoding, coordinates -> address
	kit.Handle(app, kit.OpMeta{
		Name:    "reverse",
		Group:   "read",
		Single:  true,
		Summary: "Reverse geocoding: convert coordinates to an address",
	}, reverseOp)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type searchInput struct {
	Query  string  `kit:"arg"         help:"place name or address to geocode"`
	Limit  int     `kit:"flag,inherit" help:"max results (default 5)"`
	Client *Client `kit:"inject"`
}

type reverseInput struct {
	Lat    float64 `kit:"flag" help:"latitude"`
	Lon    float64 `kit:"flag" help:"longitude"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(Location) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}
	items, err := in.Client.Search(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func reverseOp(ctx context.Context, in reverseInput, emit func(Address) error) error {
	addr, err := in.Client.Reverse(ctx, in.Lat, in.Lon)
	if err != nil {
		return mapErr(err)
	}
	return emit(*addr)
}

// --- Resolver: pure string functions, no network ---

// Classify turns an input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty nominatim reference")
	}
	if u, err := neturl.Parse(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return "place", u.RawQuery, nil
	}
	return "place", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "place":
		return "https://nominatim.openstreetmap.org/search?q=" + neturl.QueryEscape(id) + "&format=json", nil
	default:
		return "", errs.Usage("nominatim has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
