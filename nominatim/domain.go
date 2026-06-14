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
		Args: []kit.Arg{
			{Name: "lat", Help: "latitude (decimal degrees, e.g. 48.8566)"},
			{Name: "lon", Help: "longitude (decimal degrees, e.g. 2.3522)"},
		},
	}, reverseOp)

	// lookup: fetch OSM objects by prefixed ID (N=node, R=relation, W=way)
	kit.Handle(app, kit.OpMeta{
		Name:    "lookup",
		Group:   "read",
		List:    true,
		Summary: "Look up OSM objects by ID (e.g. R7444 N1234)",
	}, lookupOp)

	// status: check API health and data freshness
	kit.Handle(app, kit.OpMeta{
		Name:    "status",
		Group:   "read",
		Single:  true,
		Summary: "Check API health and data freshness",
	}, statusOp)
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
	Query   string  `kit:"arg"          help:"place name or address to geocode"`
	Limit   int     `kit:"flag,inherit" help:"max results (default 5)"`
	Country string  `kit:"flag"         help:"ISO 3166-1 alpha-2 country code to restrict results (e.g. US, FR)"`
	Client  *Client `kit:"inject"`
}

type reverseInput struct {
	Lat    string  `kit:"arg" help:"latitude (decimal degrees, e.g. 48.8566)"`
	Lon    string  `kit:"arg" help:"longitude (decimal degrees, e.g. 2.3522)"`
	Client *Client `kit:"inject"`
}

type lookupInput struct {
	IDs    []string `kit:"args" help:"OSM IDs with type prefix (e.g. R7444 N1234)"`
	Client *Client  `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(Location) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}
	items, err := in.Client.Search(ctx, in.Query, limit, in.Country)
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

func lookupOp(ctx context.Context, in lookupInput, emit func(Location) error) error {
	items, err := in.Client.Lookup(ctx, in.IDs)
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

type statusInput struct {
	Client *Client `kit:"inject"`
}

func statusOp(ctx context.Context, in statusInput, emit func(Status) error) error {
	s, err := in.Client.Status(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(*s)
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
