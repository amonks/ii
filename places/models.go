package places

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"monks.co/googlemaps"
	"monks.co/logger"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type Place struct {
	// GoogleMapsURL is the primary key in the places table.
	GoogleMapsURL string

	// GoogleMapsPlaceID is only set for businesses with an official google maps
	// presence.
	GoogleMapsPlaceID string

	// GoogleMapsBusinessStatus must be one of:
	//   - OPERATIONAL,
	//   - CLOSED_TEMPORARILY,
	//   - CLOSED_PERMANENTLY, or,
	//   - the empty string.
	GoogleMapsBusinessStatus string

	// IsPublic is true if the place should be displayed to monks.co visitors.
	IsPublic bool

	Notes  string
	Rating int

	CreatedAt string
	UpdatedAt string

	Lat string
	Lng string

	BusinessName string
	CountryCode  string
	Address      string
	Title        string

	DetailsJSON string

	EditorialSummary string
	OpeningHours     []string
	Types            []string
	PriceLevel       int
}

func (p Place) DisplayName() (string, error) {
	if p.BusinessName != "" {
		return p.BusinessName, nil
	}
	if p.Title != "" {
		return p.Title, nil
	}
	if p.Address != "" {
		return p.Address, nil
	}
	return "", fmt.Errorf("could not create display name for '%s'", p.GoogleMapsURL)
}

type model struct {
	logger.Logger
}

func NewModel() *model {
	return &model{
		Logger: logger.New("places model"),
	}
}

func (m *model) migrate(conn *sqlite.Conn) error {
	if err := sqlitex.ExecScript(conn, `
		create table if not exists places (
			google_maps_url text primary key not null,
			google_maps_place_id text,
			google_maps_business_status text,
			is_public integer,
			notes text,
			rating integer,
			created_at text,
			updated_at text,
			lat text,
			lng text,
			business_name text,
			country_code text,
			address text,
			title text,
			details_json text,
			editorial_summary text,
			opening_hours text,
			types text,
			price_level int
		);`); err != nil {
		return err
	}
	return nil
}

func (m *model) listPlaces(conn *sqlite.Conn) ([]Place, error) {
	urls := []string{}
	const query = `
		select google_maps_url
		from places;`

	onResult := func(stmt *sqlite.Stmt) error {
		urls = append(urls, stmt.ColumnText(0))
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult); err != nil {
		return nil, err
	}

	places := []Place{}
	for _, url := range urls {
		place, err := m.getPlace(conn, url)
		if err != nil {
			return places, err
		}
		if place != nil {
			places = append(places, *place)
		}
	}
	return places, nil
}

func (m *model) getPlace(conn *sqlite.Conn, googleMapsURL string) (*Place, error) {
	const query = `
		select
			google_maps_place_id,
			google_maps_business_status,
			is_public,
			notes,
			rating,
			created_at,
			updated_at,
			lat,
			lng,
			business_name,
			country_code,
			address,
			title,
			details_json,
			editorial_summary,
			opening_hours,
			types,
			price_level
		from places
		where google_maps_url = ?;`

	var place *Place
	onResult := func(stmt *sqlite.Stmt) error {
		place = &Place{
			GoogleMapsURL:            googleMapsURL,
			GoogleMapsPlaceID:        stmt.ColumnText(0),
			GoogleMapsBusinessStatus: stmt.ColumnText(1),
			IsPublic:                 stmt.ColumnInt(2) == 1,
			Notes:                    stmt.ColumnText(3),
			Rating:                   stmt.ColumnInt(4),
			CreatedAt:                stmt.ColumnText(5),
			UpdatedAt:                stmt.ColumnText(6),
			Lat:                      stmt.ColumnText(7),
			Lng:                      stmt.ColumnText(8),
			BusinessName:             stmt.ColumnText(9),
			CountryCode:              stmt.ColumnText(10),
			Address:                  stmt.ColumnText(11),
			Title:                    stmt.ColumnText(12),
			DetailsJSON:              stmt.ColumnText(13),
			EditorialSummary:         stmt.ColumnText(14),
			OpeningHours:             strings.Split(stmt.ColumnText(15), "\n"),
			Types:                    strings.Split(stmt.ColumnText(16), "\n"),
			PriceLevel:               stmt.ColumnInt(17),
		}
		return nil
	}

	if err := sqlitex.Exec(conn, query, onResult, googleMapsURL); err != nil {
		return nil, err
	}

	return place, nil
}

func (m *model) findPlaceByAddress(conn *sqlite.Conn, address string) (*Place, error) {
	const query = `
		select
			google_maps_url
		from
			places
		where
			google_maps_url like '%' || ? || '%'
			or google_maps_url like '%' || ? || '%'
			or address like '%' || ? || '%'
			or title like '%' || ? || '%'
		limit 1;`

	var url *string
	onResult := func(stmt *sqlite.Stmt) error {
		got := stmt.ColumnText(0)
		url = &got
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult, address, strings.ReplaceAll(address, " ", "+"), address, address); err != nil {
		fmt.Println("failed to match address:", address)
		return nil, err
	}
	fmt.Println("address:", address, "url:", url)
	if url == nil {
		return nil, nil
	}

	place, err := m.getPlace(conn, *url)
	if err != nil {
		return nil, err
	}

	return place, nil
}

func (m *model) insertPlace(conn *sqlite.Conn, place Place) error {
	const query = `
		insert into places
			(
				google_maps_url,
				google_maps_place_id,
				google_maps_business_status,
				is_public,
				notes,
				rating,
				created_at,
				updated_at,
				lat,
				lng,
				business_name,
				country_code,
				address,
				title,
				details_json,
				editorial_summary,
				opening_hours,
				types,
				price_level
			)
		values
			(
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?,
				?
			);`
	err := sqlitex.Exec(conn, query, nil,
		place.GoogleMapsURL,
		place.GoogleMapsPlaceID,
		place.GoogleMapsBusinessStatus,
		place.IsPublic,
		place.Notes,
		place.Rating,
		place.CreatedAt,
		place.UpdatedAt,
		place.Lat,
		place.Lng,
		place.BusinessName,
		place.CountryCode,
		place.Address,
		place.Title,
		place.DetailsJSON,
		place.EditorialSummary,
		strings.Join(place.OpeningHours, "\n"),
		strings.Join(place.Types, "\n"),
		place.PriceLevel)
	return err
}

func (m *model) updatePlace(conn *sqlite.Conn, place Place) error {
	const query = `
		update places
		set
			google_maps_place_id = ?,
			google_maps_business_status = ?,
			is_public = ?,
			notes = ?,
			rating = ?,
			updated_at = ?,
			lat = ?,
			lng = ?,
			business_name = ?,
			country_code = ?,
			address = ?,
			title = ?,
			details_json = ?,
			editorial_summary = ?,
			opening_hours = ?,
			types = ?,
			price_level = ?
		where
			google_maps_url = ?;`
	err := sqlitex.Exec(conn, query, nil,
		place.GoogleMapsPlaceID,
		place.GoogleMapsBusinessStatus,
		place.IsPublic,
		place.Notes,
		place.Rating,
		time.Now().Format(time.RFC3339),
		place.Lat,
		place.Lng,
		place.BusinessName,
		place.CountryCode,
		place.Address,
		place.Title,
		place.DetailsJSON,
		place.EditorialSummary,
		strings.Join(place.OpeningHours, "\n"),
		strings.Join(place.Types, "\n"),
		place.PriceLevel,
		place.GoogleMapsURL,
	)
	return err
}

func (m *model) importSavedPlaces(conn *sqlite.Conn) error {
	const filename = "places/saved_places.json"

	jsonFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	var savedPlaces struct {
		Features []struct {
			Properties struct {
				GoogleMapsURL string `json:"Google Maps URL"`
				Location      struct {
					Address      string `json:"Address"`
					BusinessName string `json:"Business Name"`
					CountryCode  string `json:"Country Code"`
					GeoCoords    struct {
						Latitude  string `json:"Latitude"`
						Longitude string `json:"Longitude"`
					} `json:"Geo Coordinates"`
				} `json:"Location"`
				Published string `json:"Published"`
				Title     string `json:"Title"`
				Updated   string `json:"Updated"`
			} `json:"properties"`
			Type string `json:"type"`
		} `json:"Features"`
	}
	if err := json.Unmarshal(jsonBytes, &savedPlaces); err != nil {
		return err
	}

	m.Logf("importing %d places\n", len(savedPlaces.Features))

	places := []Place{}
	for _, savedPlace := range savedPlaces.Features {
		place := Place{
			GoogleMapsURL: savedPlace.Properties.GoogleMapsURL,
			IsPublic:      true,
			Notes:         "",
			Rating:        0,
			CreatedAt:     savedPlace.Properties.Published,
			UpdatedAt:     savedPlace.Properties.Updated,
			Lat:           savedPlace.Properties.Location.GeoCoords.Latitude,
			Lng:           savedPlace.Properties.Location.GeoCoords.Longitude,
			BusinessName:  savedPlace.Properties.Location.BusinessName,
			CountryCode:   savedPlace.Properties.Location.CountryCode,
			Address:       savedPlace.Properties.Location.Address,
			Title:         savedPlace.Properties.Title,
		}
		places = append(places, place)
	}

	for _, place := range places {
		// See if place exists
		got, err := m.getPlace(conn, place.GoogleMapsURL)
		if err != nil {
			m.Logf("ERROR fetching %s", place.Title)
			return err
		}

		// Skip if it exists and we already have details
		// XXX: This means we'll never update BusinessStatus
		if got != nil && got.GoogleMapsPlaceID != "" && got.EditorialSummary != "" {
			continue
		}

		// Fetch place details
		details, err := googlemaps.GetPlaceDetailsByURL(place.GoogleMapsURL)
		if err != nil {
			m.Logf("Error getting details for %s", place.Title)
			return err
		}
		if details != nil {
			place.GoogleMapsPlaceID = details.PlaceID
			place.GoogleMapsBusinessStatus = details.BusinessStatus
			place.DetailsJSON = details.DetailsJSON
			place.EditorialSummary = details.EditorialSummary
			place.OpeningHours = details.OpeningHours
			place.Types = details.Types
			place.PriceLevel = details.PriceLevel
		}

		if got == nil {
			m.Logf("inserting %s", place.Title)
			if err := m.insertPlace(conn, place); err != nil {
				return err
			}
		}

		if got != nil {
			m.Logf("updating %s", place.Title)

			// careful -- only set fields that are
			// controlled by google maps
			got.GoogleMapsBusinessStatus = place.GoogleMapsBusinessStatus
			got.DetailsJSON = place.DetailsJSON
			got.EditorialSummary = place.EditorialSummary
			got.OpeningHours = place.OpeningHours
			got.Types = place.Types
			got.PriceLevel = place.PriceLevel

			if err := m.updatePlace(conn, *got); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *model) annotatePeoplesPlaces(conn *sqlite.Conn) error {
	const filename = "places/people.csv"

	csvFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvReader := csv.NewReader(csvFile)
	rows, err := csvReader.ReadAll()
	if err != nil {
		return err
	}

	type record struct {
		Title string
		Note  string
		URL   string
	}

	records := []record{}
	for _, row := range rows {
		records = append(records, record{
			Title: row[0],
			Note:  row[1],
			URL:   row[2],
		})
	}

	for _, record := range records[1:] {
		if record.Title == "" {
			continue
		}
		place, err := m.findPlaceByAddress(conn, record.Title)
		if err != nil {
			return err
		}
		if place == nil {
			return fmt.Errorf("could not find place: '%s'", record.Title)
		}
		person := record.Note
		if person == "Me" {
			person = "I"
		}
		note := fmt.Sprintf("%s was here", person)
		if strings.Contains(place.Notes, note) {
			continue
		}
		if place.Notes != "" {
			place.Notes += "\n\n"
		}
		place.Notes += note
		if err := m.updatePlace(conn, *place); err != nil {
			return err
		}
	}

	return nil
}

func (p *Place) IsOperational() bool {
	return p.GoogleMapsBusinessStatus == "" || p.GoogleMapsBusinessStatus == "OPERATIONAL"
}
