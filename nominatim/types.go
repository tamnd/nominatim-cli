package nominatim

import (
	"fmt"
)

// Location is one result from the Nominatim search (forward geocoding) endpoint.
type Location struct {
	Rank        int     `json:"rank"`
	PlaceID     int64   `json:"place_id"`
	Name        string  `kit:"id" json:"name"`
	DisplayName string  `json:"display_name"`
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	Type        string  `json:"type"`       // e.g. "administrative", "city"
	Class       string  `json:"class"`      // e.g. "boundary", "place"
	Importance  float64 `json:"importance"` // 0.0–1.0 relevance score
	OsmType     string  `json:"osm_type"`   // "relation", "node", or "way"
	OsmID       int64   `json:"osm_id"`
	URL         string  `json:"url"` // https://www.openstreetmap.org/{type}/{id}
}

// Address is the result from the Nominatim reverse geocoding endpoint.
type Address struct {
	DisplayName string `kit:"id" json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Road        string `json:"road,omitempty"`
	City        string `json:"city,omitempty"` // city || town || village fallback
	State       string `json:"state,omitempty"`
	Country     string `json:"country,omitempty"`
	PostCode    string `json:"postcode,omitempty"`
}

// Status is the result from the Nominatim /status endpoint.
type Status struct {
	Status          int    `json:"status"`
	Message         string `json:"message"`
	DataUpdated     string `json:"data_updated"`
	SoftwareVersion string `json:"software_version"`
	DatabaseVersion string `json:"database_version"`
}

// unexported: only used for JSON decode from the Nominatim API

// rawLocation is the JSON shape from /search. lat and lon are strings in the
// API response, not floats.
type rawLocation struct {
	PlaceID     int64   `json:"place_id"`
	OsmType     string  `json:"osm_type"`
	OsmID       int64   `json:"osm_id"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	Class       string  `json:"class"`
	Type        string  `json:"type"`
	Importance  float64 `json:"importance"`
}

func (r rawLocation) toLocation(rank int) (Location, error) {
	if r.Lat == "" {
		return Location{}, fmt.Errorf("missing lat in result")
	}
	return Location{
		Rank:        rank,
		PlaceID:     r.PlaceID,
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Lat:         r.Lat,
		Lon:         r.Lon,
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
		Lat:         r.Lat,
		Lon:         r.Lon,
		Road:        r.Address.Road,
		City:        city,
		State:       r.Address.State,
		Country:     r.Address.Country,
		PostCode:    r.Address.Postcode,
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
