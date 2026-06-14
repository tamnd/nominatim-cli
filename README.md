# nominatim

Geocoding CLI for [Nominatim](https://nominatim.openstreetmap.org) (OpenStreetMap). No API key required.

`nominatim` converts place names to coordinates (forward geocoding) and coordinates to addresses (reverse geocoding) using the free public Nominatim API. It is a single pure-Go binary that respects the Nominatim usage policy: identifies itself with a proper User-Agent and paces requests to stay within the 1 req/s limit.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
Nominatim as `nominatim://` URIs.

## Install

```bash
go install github.com/tamnd/nominatim-cli/cmd/nominatim@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/nominatim-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/nominatim:latest --help
```

## Usage

```bash
# Forward geocoding: place name -> coordinates
nominatim search "Paris, France"
nominatim search "Eiffel Tower" --limit 3

# Reverse geocoding: coordinates -> address
nominatim reverse --lat 48.8566 --lon 2.3522

# Control output format
nominatim search "Tokyo" -o json
nominatim search "London" --fields rank,display_name,lat,lon
nominatim reverse --lat 35.6762 --lon 139.6503 --template '{{.City}}, {{.Country}}'
```

Every command shares one output contract: `-o table|json|jsonl|csv|tsv|url|raw`,
`--fields` to pick columns, `--template` for a custom line, and `-n` to limit.

## Commands

| Command | Description |
|---------|-------------|
| `nominatim search <query>` | Forward geocoding: place name or address to coordinates |
| `nominatim reverse --lat LAT --lon LON` | Reverse geocoding: coordinates to address |

## Usage policy

Nominatim is a free service run by the OpenStreetMap Foundation. Using it responsibly:

- Every request identifies itself as `nominatim-cli` with a contact URL.
- Requests are paced at 1100 ms minimum gap (10% headroom above the 1 req/s hard limit).
- Bulk geocoding is not supported. For batch workloads, consider self-hosting Nominatim.

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents:

```bash
nominatim serve --addr :7777    # GET /v1/search?query=Paris  returns NDJSON
nominatim mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`nominatim` registers a `nominatim` domain the way a program registers a
database driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/nominatim-cli/nominatim"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `nominatim://` URIs without knowing anything about Nominatim:

```bash
ant get nominatim://place/Paris
ant url nominatim://place/Paris
```

## Development

```
cmd/nominatim/   thin main: hands cli.NewApp to kit.Run
cli/             assembles the kit App from the nominatim domain
nominatim/       the library: HTTP client, data models, and domain.go (the driver)
docs/            documentation site
```

```bash
make build      # ./bin/nominatim
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser:

```bash
git tag v0.1.0
git push --tags
```

## License

Apache-2.0. See [LICENSE](LICENSE).
