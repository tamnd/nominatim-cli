package nominatim_test

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/nominatim-cli/nominatim"
)

const fakeSearchJSON = `[
  {
    "place_id": 162639773,
    "osm_type": "relation",
    "osm_id": 7444,
    "display_name": "Paris, Île-de-France, France métropolitaine, France",
    "lat": "48.8534951",
    "lon": "2.3483915",
    "class": "boundary",
    "type": "administrative",
    "importance": 0.9306897
  },
  {
    "place_id": 298950971,
    "osm_type": "relation",
    "osm_id": 71525,
    "display_name": "Paris, Texas, United States",
    "lat": "33.6617962",
    "lon": "-95.5554541",
    "class": "boundary",
    "type": "administrative",
    "importance": 0.58
  }
]`

const fakeReverseJSON = `{
  "place_id": 297362933,
  "display_name": "Tour Eiffel, Champ de Mars, Paris, Île-de-France, France",
  "lat": "48.8566101",
  "lon": "2.3514992",
  "address": {
    "attraction": "Tour Eiffel",
    "road": "Champ de Mars",
    "city": "Paris",
    "state": "Île-de-France",
    "country": "France",
    "country_code": "fr",
    "postcode": "75007"
  }
}`

func newTestClient(ts *httptest.Server) *nominatim.Client {
	cfg := nominatim.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	return nominatim.NewClient(cfg)
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func TestSearchSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "Paris", 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestSearchParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Search(context.Background(), "Paris", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	// first item
	got := items[0]
	if got.Rank != 1 {
		t.Errorf("items[0].Rank = %d, want 1", got.Rank)
	}
	if got.DisplayName != "Paris, Île-de-France, France métropolitaine, France" {
		t.Errorf("items[0].DisplayName = %q", got.DisplayName)
	}
	if !approxEqual(got.Lat, 48.8534951, 0.001) {
		t.Errorf("items[0].Lat = %f, want ~48.8534951", got.Lat)
	}
	if !approxEqual(got.Lon, 2.3483915, 0.001) {
		t.Errorf("items[0].Lon = %f, want ~2.3483915", got.Lon)
	}
	wantURL := "https://www.openstreetmap.org/relation/7444"
	if got.URL != wantURL {
		t.Errorf("items[0].URL = %q, want %q", got.URL, wantURL)
	}

	// second item
	got2 := items[1]
	if got2.Rank != 2 {
		t.Errorf("items[1].Rank = %d, want 2", got2.Rank)
	}
	if got2.DisplayName != "Paris, Texas, United States" {
		t.Errorf("items[1].DisplayName = %q", got2.DisplayName)
	}
}

func TestSearchLimitRespected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Search(context.Background(), "Paris", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
}

func TestSearchRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	cfg := nominatim.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := nominatim.NewClient(cfg)

	_, err := c.Search(context.Background(), "Paris", 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestReverseParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeReverseJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	addr, err := c.Reverse(context.Background(), 48.8566, 2.3522)
	if err != nil {
		t.Fatal(err)
	}
	if addr.DisplayName != "Tour Eiffel, Champ de Mars, Paris, Île-de-France, France" {
		t.Errorf("DisplayName = %q", addr.DisplayName)
	}
	if addr.City != "Paris" {
		t.Errorf("City = %q, want Paris", addr.City)
	}
	if addr.Country != "France" {
		t.Errorf("Country = %q, want France", addr.Country)
	}
	if addr.CountryCode != "fr" {
		t.Errorf("CountryCode = %q, want fr", addr.CountryCode)
	}
	if addr.Road != "Champ de Mars" {
		t.Errorf("Road = %q, want Champ de Mars", addr.Road)
	}
	if addr.Postcode != "75007" {
		t.Errorf("Postcode = %q, want 75007", addr.Postcode)
	}
	if !approxEqual(addr.Lat, 48.8566101, 0.001) {
		t.Errorf("Lat = %f, want ~48.8566101", addr.Lat)
	}
	if !approxEqual(addr.Lon, 2.3514992, 0.001) {
		t.Errorf("Lon = %f, want ~2.3514992", addr.Lon)
	}
}
