package nominatim

import (
	"fmt"
	"strconv"
)

// Location is one result from the Nominatim search (forward geocoding) endpoint.
type Location struct {
	Rank        int     `json:"rank"`
	PlaceID     int64   `json:"place_id"`
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Type        string  `json:"type"`       // e.g. "administrative", "city"
	Class       string  `json:"class"`      // e.g. "boundary", "place"
	Importance  float64 `json:"importance"` // 0.0–1.0 relevance score
	OsmType     string  `json:"osm_type"`   // "relation", "node", or "way"
	OsmID       int64   `json:"osm_id"`
	URL         string  `json:"url"` // https://www.openstreetmap.org/{type}/{id}
}

// Address is the result from the Nominatim reverse geocoding endpoint.
type Address struct {
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	House       string  `json:"house,omitempty"` // house_number
	Road        string  `json:"road,omitempty"`
	City        string  `json:"city,omitempty"` // city || town || village fallback
	State       string  `json:"state,omitempty"`
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Postcode    string  `json:"postcode,omitempty"`
}

// unexported: only used for JSON decode from the Nominatim API

// rawLocation is the JSON shape from /search. lat and lon are strings in the
// API response, not floats.
type rawLocation struct {
	PlaceID     int64   `json:"place_id"`
	OsmType     string  `json:"osm_type"`
	OsmID       int64   `json:"osm_id"`
	DisplayName string  `json:"display_name"`
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	Class       string  `json:"class"`
	Type        string  `json:"type"`
	Importance  float64 `json:"importance"`
}

func (r rawLocation) toLocation(rank int) (Location, error) {
	lat, err := strconv.ParseFloat(r.Lat, 64)
	if err != nil {
		return Location{}, fmt.Errorf("parse lat %q: %w", r.Lat, err)
	}
	lon, err := strconv.ParseFloat(r.Lon, 64)
	if err != nil {
		return Location{}, fmt.Errorf("parse lon %q: %w", r.Lon, err)
	}
	return Location{
		Rank:        rank,
		PlaceID:     r.PlaceID,
		DisplayName: r.DisplayName,
		Lat:         lat,
		Lon:         lon,
		Type:        r.Type,
		Class:       r.Class,
		Importance:  r.Importance,
		OsmType:     r.OsmType,
		OsmID:       r.OsmID,
		URL:         osmURL(r.OsmType, r.OsmID),
	}, nil
}

// rawReverse is the JSON shape from /reverse.
type rawReverse struct {
	PlaceID     int64      `json:"place_id"`
	DisplayName string     `json:"display_name"`
	Lat         string     `json:"lat"`
	Lon         string     `json:"lon"`
	Address     rawAddress `json:"address"`
}

func (r rawReverse) toAddress() (*Address, error) {
	lat, err := strconv.ParseFloat(r.Lat, 64)
	if err != nil {
		return nil, fmt.Errorf("parse lat %q: %w", r.Lat, err)
	}
	lon, err := strconv.ParseFloat(r.Lon, 64)
	if err != nil {
		return nil, fmt.Errorf("parse lon %q: %w", r.Lon, err)
	}
	city := r.Address.City
	if city == "" {
		city = r.Address.Town
	}
	if city == "" {
		city = r.Address.Village
	}
	if city == "" {
		city = r.Address.Municipality
	}
	return &Address{
		DisplayName: r.DisplayName,
		Lat:         lat,
		Lon:         lon,
		House:       r.Address.HouseNumber,
		Road:        r.Address.Road,
		City:        city,
		State:       r.Address.State,
		Country:     r.Address.Country,
		CountryCode: r.Address.CountryCode,
		Postcode:    r.Address.Postcode,
	}, nil
}

// rawAddress is the address subobject inside /reverse responses. Many fields
// are optional and depend on the zoom level and the type of place.
type rawAddress struct {
	HouseNumber  string `json:"house_number"`
	Road         string `json:"road"`
	City         string `json:"city"`
	Town         string `json:"town"`         // fallback if city is empty
	Village      string `json:"village"`      // fallback if town is empty
	Municipality string `json:"municipality"` // fallback if village is empty
	State        string `json:"state"`
	Country      string `json:"country"`
	CountryCode  string `json:"country_code"`
	Postcode     string `json:"postcode"`
}
