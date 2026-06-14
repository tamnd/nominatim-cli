package nominatim_test

import (
	"context"
	"fmt"
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
    "name": "Paris",
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
    "name": "Paris",
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

func TestSearchSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "Paris", 1, "")
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
	items, err := c.Search(context.Background(), "Paris", 0, "")
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
	if got.Name != "Paris" {
		t.Errorf("items[0].Name = %q, want Paris", got.Name)
	}
	if got.DisplayName != "Paris, Île-de-France, France métropolitaine, France" {
		t.Errorf("items[0].DisplayName = %q", got.DisplayName)
	}
	if got.Lat != "48.8534951" {
		t.Errorf("items[0].Lat = %q, want 48.8534951", got.Lat)
	}
	if got.Lon != "2.3483915" {
		t.Errorf("items[0].Lon = %q, want 2.3483915", got.Lon)
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
	items, err := c.Search(context.Background(), "Paris", 1, "")
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

	_, err := c.Search(context.Background(), "Paris", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

const fakeLookupJSON = `[
  {
    "place_id": 162639773,
    "osm_type": "relation",
    "osm_id": 7444,
    "name": "Paris",
    "display_name": "Paris, Île-de-France, France métropolitaine, France",
    "lat": "48.8534951",
    "lon": "2.3483915",
    "class": "boundary",
    "type": "administrative",
    "importance": 0.9306897
  }
]`

func TestLookupSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, fakeLookupJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Lookup(context.Background(), []string{"R7444"})
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent on lookup")
	}
}

func TestLookupParsesItems(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, fakeLookupJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Lookup(context.Background(), []string{"R7444"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	got := items[0]
	if got.Rank != 1 {
		t.Errorf("Rank = %d, want 1", got.Rank)
	}
	if got.DisplayName != "Paris, Île-de-France, France métropolitaine, France" {
		t.Errorf("DisplayName = %q", got.DisplayName)
	}
	if got.OsmType != "relation" {
		t.Errorf("OsmType = %q, want relation", got.OsmType)
	}
	if got.OsmID != 7444 {
		t.Errorf("OsmID = %d, want 7444", got.OsmID)
	}
	wantURL := "https://www.openstreetmap.org/relation/7444"
	if got.URL != wantURL {
		t.Errorf("URL = %q, want %q", got.URL, wantURL)
	}
	// Check that osmnodes parameter was sent
	if !containsStr(gotQuery, "osmnodes") {
		t.Errorf("query %q does not contain osmnodes", gotQuery)
	}
}

func TestLookupEmptyIDs(t *testing.T) {
	c := nominatim.NewClient(nominatim.DefaultConfig())
	_, err := c.Lookup(context.Background(), nil)
	if err == nil {
		t.Error("expected error for empty ID list")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestReverseParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeReverseJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	addr, err := c.Reverse(context.Background(), "48.8566", "2.3522")
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
	if addr.Road != "Champ de Mars" {
		t.Errorf("Road = %q, want Champ de Mars", addr.Road)
	}
	if addr.PostCode != "75007" {
		t.Errorf("PostCode = %q, want 75007", addr.PostCode)
	}
	if addr.Lat != "48.8566101" {
		t.Errorf("Lat = %q, want 48.8566101", addr.Lat)
	}
	if addr.Lon != "2.3514992" {
		t.Errorf("Lon = %q, want 2.3514992", addr.Lon)
	}
}

func TestReverseSendsLatLon(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, fakeReverseJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Reverse(context.Background(), "40.7128", "-74.0060")
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(gotQuery, "lat=40.7128") {
		t.Errorf("query %q does not contain lat=40.7128", gotQuery)
	}
	if !containsStr(gotQuery, "lon=-74.0060") {
		t.Errorf("query %q does not contain lon=-74.0060", gotQuery)
	}
}

func TestSearchCountryParam(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "Lyon", 5, "fr")
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(gotQuery, "countrycodes=fr") {
		t.Errorf("query %q does not contain countrycodes=fr", gotQuery)
	}
}

const fakeStatusJSON = `{
  "status": 0,
  "message": "OK",
  "data_updated": "2024-01-15T01:00:01+00:00",
  "software_version": "4.4.0-0",
  "database_version": "4.4.0-0"
}`

func TestStatusParsesOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeStatusJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	s, err := c.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != 0 {
		t.Errorf("Status = %d, want 0", s.Status)
	}
	if s.Message != "OK" {
		t.Errorf("Message = %q, want OK", s.Message)
	}
	if s.DataUpdated == "" {
		t.Error("DataUpdated is empty")
	}
}
