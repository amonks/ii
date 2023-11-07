package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"monks.co/pkg/database"
	"monks.co/pkg/googlemaps"
)

type Place struct {
	// GoogleMapsURL is the primary key in the places table.
	GoogleMapsURL string `gorm:"column:google_maps_url;primaryKey"`

	// GoogleMapsPlaceID is only set for businesses with an official google maps
	// presence.
	GoogleMapsPlaceID string `gorm:"column:google_maps_place_id"`

	// GoogleMapsBusinessStatus must be one of:
	//   - OPERATIONAL,
	//   - CLOSED_TEMPORARILY,
	//   - CLOSED_PERMANENTLY, or,
	//   - the empty string.
	GoogleMapsBusinessStatus string `gorm:"column:google_maps_business_status"`

	// IsPublic is true if the place should be displayed to monks.co visitors.
	IsPublic bool `gorm:"column:is_public"`

	Notes  string `gorm:"column:notes"`
	Rating int    `gorm:"column:rating"`

	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`

	Lat string `gorm:"column:lat"`
	Lng string `gorm:"column:lng"`

	BusinessName string `gorm:"column:business_name"`
	CountryCode  string `gorm:"column:country_code"`
	Address      string `gorm:"column:address"`
	Title        string `gorm:"column:title"`

	DetailsJSON string `gorm:"column:details_json"`

	EditorialSummary string   `gorm:"column:editorial_summary"`
	OpeningHours     []string `gorm:"column:opening_hours;serializer:json"`
	Types            []string `gorm:"column:types;serializer:json"`
	PriceLevel       int      `gorm:"column:price_level"`
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
	*database.DB
}

func NewModel() (*model, error) {
	db, err := database.OpenFromDataFolder("map")
	if err != nil {
		return nil, err
	}
	return &model{db}, nil
}

func (m *model) listPlaces() ([]Place, error) {
	places := []Place{}
	tx := m.DB.Table("places").Find(&places)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return places, nil
}

func (m *model) getPlace(googleMapsURL string) (*Place, error) {
	var place Place
	if err := m.DB.
		Table("places").
		Where(&Place{GoogleMapsURL: googleMapsURL}).
		First(&place).
		Error; err != nil {
		return nil, err
	}
	return &place, nil
}

func (m *model) findPlaceByAddress(address string) (*Place, error) {
	var place Place
	if err := m.DB.
		Where("google_maps_url like '%' || ? || '%'", address).
		Or("google_maps_url like '%' || ? || '%''", strings.ReplaceAll(address, " ", "+")).
		Or("address like '%' || ? || '%'", address).
		Or("title like '%' || ? || '%'", address).
		First(&place).
		Error; err != nil {
		return nil, err
	}
	return &place, nil
}

func (m *model) insertPlace(place Place) error {
	if err := m.DB.Create(place).Error; err != nil {
		return err
	}
	return nil
}

func (m *model) updatePlace(place Place) error {
	if err := m.DB.Save(place).Error; err != nil {
		return err
	}
	return nil
}

func (m *model) importSavedPlaces() error {
	filename := filepath.Join(os.Getenv("MONKS_ROOT"), "apps", "map", "saved_places.json")
	jsonBytes, err := os.ReadFile(filename)
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

	fmt.Printf("importing %d places\n", len(savedPlaces.Features))

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
		got, err := m.getPlace(place.GoogleMapsURL)
		if err != nil {
			fmt.Printf("ERROR fetching %s\n", place.Title)
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
			fmt.Printf("Error getting details for %s\n", place.Title)
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
			fmt.Printf("inserting %s\n", place.Title)
			if err := m.insertPlace(place); err != nil {
				return err
			}
		}

		if got != nil {
			fmt.Printf("updating %s\n", place.Title)

			// careful -- only set fields that are
			// controlled by google maps
			got.GoogleMapsBusinessStatus = place.GoogleMapsBusinessStatus
			got.DetailsJSON = place.DetailsJSON
			got.EditorialSummary = place.EditorialSummary
			got.OpeningHours = place.OpeningHours
			got.Types = place.Types
			got.PriceLevel = place.PriceLevel

			if err := m.updatePlace(*got); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *model) annotatePeoplesPlaces() error {
	filename := filepath.Join(os.Getenv("MONKS_ROOT"), "apps", "places", "people.csv")

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
		place, err := m.findPlaceByAddress(record.Title)
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
		if err := m.updatePlace(*place); err != nil {
			return err
		}
	}

	return nil
}

func (p *Place) IsOperational() bool {
	return p.GoogleMapsBusinessStatus == "" || p.GoogleMapsBusinessStatus == "OPERATIONAL"
}
