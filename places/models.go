package places

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"co.monks.monks.co/googlemaps"
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

func listPlaces(conn *sqlite.Conn) ([]Place, error) {
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
		place, err := getPlace(conn, url)
		if err != nil {
			return places, err
		}
		if place != nil {
			places = append(places, *place)
		}
	}
	return places, nil
}

func getPlace(conn *sqlite.Conn, googleMapsURL string) (*Place, error) {
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
			title
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
		}
		return nil
	}

	if err := sqlitex.Exec(conn, query, onResult, googleMapsURL); err != nil {
		return nil, err
	}

	return place, nil
}

func findPlaceByAddress(conn *sqlite.Conn, address string) (*Place, error) {
	const query = `
		select
			google_maps_url
		from
			places
		where
			true
		limit 1;`

	var url *string
	onResult := func(stmt *sqlite.Stmt) error {
		got := stmt.ColumnText(0)
		url = &got
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult); err != nil {
		return nil, err
	}
	if url == nil {
		return nil, nil
	}

	place, err := getPlace(conn, *url)
	if err != nil {
		return nil, err
	}

	return place, nil
}

func insertPlace(conn *sqlite.Conn, place Place) error {
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
				title
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
		place.Title)
	return err
}

func updatePlace(conn *sqlite.Conn, place Place) error {
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
			title = ?
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
		place.GoogleMapsURL,
	)
	return err
}

func importSavedPlaces(conn *sqlite.Conn) error {
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

	log.Printf("importing %d places\n", len(savedPlaces.Features))

	places := []Place{}
	for _, savedPlace := range savedPlaces.Features {
		log.Printf("importing %s", savedPlace.Properties.GoogleMapsURL)
		place := Place{
			GoogleMapsURL:            savedPlace.Properties.GoogleMapsURL,
			IsPublic:                 true,
			Notes:                    "",
			Rating:                   0,
			CreatedAt:                savedPlace.Properties.Published,
			UpdatedAt:                savedPlace.Properties.Updated,
			Lat:                      savedPlace.Properties.Location.GeoCoords.Latitude,
			Lng:                      savedPlace.Properties.Location.GeoCoords.Longitude,
			BusinessName:             savedPlace.Properties.Location.BusinessName,
			CountryCode:              savedPlace.Properties.Location.CountryCode,
			Address:                  savedPlace.Properties.Location.Address,
			Title:                    savedPlace.Properties.Title,
		}
		details, err := googlemaps.GetPlaceDetailsByURL(savedPlace.Properties.GoogleMapsURL)
		if err != nil {
			return err
		}
		if details != nil {
			place.GoogleMapsPlaceID = details.PlaceID
			place.GoogleMapsBusinessStatus = details.BusinessStatus
		}
		places = append(places, place)
	}

	for _, place := range places {
		log.Printf("fetching %s", place.GoogleMapsURL)
		got, err := getPlace(conn, place.GoogleMapsURL)
		if err != nil {
			log.Printf("ERROR fetching %s", place.GoogleMapsURL)
			return err
		}
		if got != nil {
			log.Printf("ERROR fetching [already exists] %s", place.GoogleMapsURL)
			continue
		}
		log.Printf("inserting %s", place.GoogleMapsURL)
		if err := insertPlace(conn, place); err != nil {
			return err
		}
	}
	return nil
}

func annotatePeoplesPlaces(conn *sqlite.Conn) error {
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
		place, err := findPlaceByAddress(conn, record.Title)
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
		if err := updatePlace(conn, *place); err != nil {
			return err
		}
	}

	return nil
}
