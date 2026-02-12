package googlemaps

import "monks.co/pkg/requireenv"

var placesBackendAPIKey = requireenv.Require("GOOGLE_PLACES_BACKEND_API_KEY")
