package googlemaps

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"monks.co/credentials"
)

type Place struct {
	PlaceID          string
	BusinessStatus   string
	Address          string
	Lat              string
	Lng              string
	URL              string
	Name             string
	PriceLevel       int
	Types            []string
	OpeningHours     []string
	EditorialSummary string
	DetailsJSON      string
}

func GetPlaceDetailsByURL(u string) (*Place, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	cid := parsed.Query().Get("cid")
	if cid == "" {
		return nil, nil
	}
	place, err := GetPlaceDetailsByCID(cid)
	return &place, err
}

func GetPlaceDetailsByPlaceId(placeId string) (Place, error) {
	return getPlaceDetails("place_id", placeId)
}

func GetPlaceDetailsByCID(cid string) (Place, error) {
	return getPlaceDetails("cid", cid)
}

func getPlaceDetails(key, value string) (Place, error) {
	apiKey := credentials.PlacesBackendAPIKey
	fieldList := strings.Join([]string{
		"place_id",
		"formatted_address",
		"business_status",
		"geometry/location/lat",
		"geometry/location/lng",
		"url",
		"name",
		"opening_hours",
		"type",
		"editorial_summary",
		"price_level",
	}, ",")
	fields := url.QueryEscape(fieldList)
	url := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/details/json?fields=%s&%s=%s&key=%s", fields, key, value, apiKey)

	var place Place

	resp, err := http.Get(url)
	if err != nil {
		return place, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		buf := new(strings.Builder)
		_, err := io.Copy(buf, resp.Body)
		if err != nil {
			return place, err
		}
		return place, fmt.Errorf("google maps error [%d]: %s", resp.StatusCode, buf.String())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return place, err
	}

	var response struct {
		Result struct {
			Name              string   `json:"name"`
			PlaceID           string   `json:"place_id"`
			URL               string   `json:"url"`
			Types             []string `json:"types"`
			PriceLevel        int      `json:"price_level"`
			BusinessStatus    string   `json:"business_status"`
			FormatteddAddress string   `json:"formatted_address"`

			OpeningHours struct {
				WeekdayText []string `json:"weekday_text"`
				Periods     []struct {
					Open struct {
					} `json:"open"`
					Close struct {
						Day  int    `json:"day"`
						Time string `json:"time"`
					} `json:"close"`
				} `json:"periods"`
			} `json:"opening_hours"`

			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`

			EditorialSummary struct {
				Language string `json:"language"`
				Overview string `json:"overview"`
			} `json:"editorial_summary"`
		} `json:"result"`
		Status string `json:"status"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return place, err
	}

	detailsJson, err := json.Marshal(response)
	if err != nil {
		return place, err
	}

	place.PlaceID = response.Result.PlaceID
	place.BusinessStatus = response.Result.BusinessStatus
	place.Address = response.Result.FormatteddAddress
	place.Lat = strconv.FormatFloat(response.Result.Geometry.Location.Lat, 'f', -1, 64)
	place.Lng = strconv.FormatFloat(response.Result.Geometry.Location.Lng, 'f', -1, 64)
	place.URL = response.Result.URL
	place.Name = response.Result.Name
	place.PriceLevel = response.Result.PriceLevel
	place.Types = response.Result.Types
	place.EditorialSummary = response.Result.EditorialSummary.Overview
	place.OpeningHours = response.Result.OpeningHours.WeekdayText
	place.DetailsJSON = string(detailsJson)

	return place, nil
}
